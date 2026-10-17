package service

// P2-26 后半：快照改为 DB 侧分页后，必须保证「分页结果拼起来 == 全量结果」。
// 这类改写最容易出的错是边界（跨类型交界处重复或漏行），所以这里直接按小页翻完再比对。

import (
	"context"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

// seedMixed 造一个混合项目：2 个分组、5 个接口、3 个文档、1 个环境 + 1 个变量。
func seedMixed(t *testing.T, f *varFixture) {
	t.Helper()
	ctx := context.Background()
	var groupIDs []uint64
	for _, title := range []string{"用户", "订单"} {
		g, err := f.s.CreateGroup(ctx, f.admin.ID, f.admin.Type, dto.GroupCreateReq{
			ProjectID: f.proj.ID, Title: title, Type: model.GroupTypeAPI,
		})
		if err != nil {
			t.Fatalf("CreateGroup %s: %v", title, err)
		}
		groupIDs = append(groupIDs, g.ID)
	}
	for i := 0; i < 5; i++ {
		if _, err := f.s.CreateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
			ProjectID: f.proj.ID, GroupID: groupIDs[i%2], ApiName: "接口" + itoaInt(i),
			HttpMethodType: "GET", URL: "/v1/x",
		}); err != nil {
			t.Fatalf("CreateApi #%d: %v", i, err)
		}
	}
	for i := 0; i < 3; i++ {
		if _, err := f.s.CreateDoc(ctx, f.admin.ID, f.admin.Type, dto.DocCreateReq{
			ProjectID: f.proj.ID, GroupID: groupIDs[0], Title: "文档" + itoaInt(i), Content: "# 正文",
		}); err != nil {
			t.Fatalf("CreateDoc #%d: %v", i, err)
		}
	}
	// varFixture 自带 dev/test 两个环境，这里再加一个变量即可覆盖变量类型
	val := "v"
	if _, err := f.s.CreateVariable(ctx, f.admin.ID, f.admin.Type, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID, Name: "kv", Value: &val,
	}); err != nil {
		t.Fatalf("CreateVariable: %v", err)
	}
}

// TestSyncSnapshotPagingMatchesFull 逐页翻完 == 一次取全量（顺序与内容都一致）。
func TestSyncSnapshotPagingMatchesFull(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	seedMixed(t, f)
	ctx := context.Background()

	full, err := f.s.SyncSnapshot(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, 1000)
	if err != nil {
		t.Fatalf("SyncSnapshot(full): %v", err)
	}
	if len(full.Items) < 11 {
		t.Fatalf("混合项目应至少 11 条（2 组 + 5 接口 + 3 文档 + 1 变量，环境另计），got %d", len(full.Items))
	}
	if full.HasMore {
		t.Fatal("一次取全量时 has_more 应为 false")
	}

	// 用 3 条一页翻完
	const pageSize = 3
	var paged []SyncItem
	offset := 0
	for pages := 0; ; pages++ {
		if pages > 50 {
			t.Fatal("分页没有收敛（可能 has_more 永远为 true）")
		}
		p, err := f.s.SyncSnapshot(ctx, f.admin.ID, f.admin.Type, f.proj.ID, offset, pageSize)
		if err != nil {
			t.Fatalf("SyncSnapshot(offset=%d): %v", offset, err)
		}
		if len(p.Items) > pageSize {
			t.Fatalf("单页返回 %d 条，超过 limit=%d", len(p.Items), pageSize)
		}
		paged = append(paged, p.Items...)
		if !p.HasMore {
			break
		}
		if len(p.Items) == 0 {
			t.Fatal("has_more=true 却返回空页（会导致客户端死循环）")
		}
		offset += len(p.Items)
	}

	if len(paged) != len(full.Items) {
		t.Fatalf("分页总条数 %d ≠ 全量 %d", len(paged), len(full.Items))
	}
	for i := range full.Items {
		if paged[i].Type != full.Items[i].Type || paged[i].UID != full.Items[i].UID {
			t.Fatalf("第 %d 条不一致：分页=(%s,%s) 全量=(%s,%s)",
				i, paged[i].Type, paged[i].UID, full.Items[i].Type, full.Items[i].UID)
		}
	}

	// 越界 offset：空结果、has_more=false（客户端据此结束）
	tail, err := f.s.SyncSnapshot(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 10000, pageSize)
	if err != nil {
		t.Fatalf("SyncSnapshot(越界): %v", err)
	}
	if len(tail.Items) != 0 || tail.HasMore {
		t.Fatalf("越界 offset 应返回空页且 has_more=false，got %d 条 has_more=%v", len(tail.Items), tail.HasMore)
	}
	if tail.Offset != 10000 {
		t.Fatalf("应回显请求的 offset，got %d", tail.Offset)
	}
}

// TestSyncSnapshotCrossTypeBoundary 页边界正好落在类型交界处时不能重复/漏行。
func TestSyncSnapshotCrossTypeBoundary(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()
	// 只造 2 个分组 + 1 个接口：第 1 页（size=2）全是分组，第 2 页从接口开始
	for _, title := range []string{"A", "B"} {
		if _, err := f.s.CreateGroup(ctx, f.admin.ID, f.admin.Type, dto.GroupCreateReq{
			ProjectID: f.proj.ID, Title: title, Type: model.GroupTypeAPI,
		}); err != nil {
			t.Fatalf("CreateGroup: %v", err)
		}
	}
	if _, err := f.s.CreateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: "接口", HttpMethodType: "GET", URL: "/v1/x",
	}); err != nil {
		t.Fatalf("CreateApi: %v", err)
	}

	p1, err := f.s.SyncSnapshot(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, 2)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(p1.Items) != 2 || p1.Items[0].Type != model.SyncTypeGroup || p1.Items[1].Type != model.SyncTypeGroup {
		t.Fatalf("第 1 页应是 2 个分组，got %+v", p1.Items)
	}
	if !p1.HasMore {
		t.Fatal("还有接口没取，has_more 应为 true")
	}
	p2, err := f.s.SyncSnapshot(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 2, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	// 第 3 条起是接口/文档/变量/环境：不应再出现分组，也不能重复第 1 页的内容
	for _, it := range p2.Items {
		if it.Type == model.SyncTypeGroup {
			t.Fatalf("第 2 页不应再出现分组（重复行）: %+v", it)
		}
	}
	if len(p2.Items) == 0 {
		t.Fatal("交界页不应为空")
	}
}
