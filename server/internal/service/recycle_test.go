package service

import (
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

// apiIDs 返回项目下正常（deleted=0）或回收站（deleted=1）中的接口 id。
func apiIDs(t *testing.T, f *varFixture, deleted int8) []uint64 {
	t.Helper()
	res, err := f.s.ListApis(f.ctx, f.admin.ID, f.admin.Type, dto.ApiListQuery{
		ProjectID: f.proj.ID, Deleted: deleted, PageSize: 200,
	})
	if err != nil {
		t.Fatalf("ListApis(deleted=%d): %v", deleted, err)
	}
	out := make([]uint64, 0, len(res.Items))
	for _, it := range res.Items {
		out = append(out, it.ID)
	}
	return out
}

func containsID(ids []uint64, id uint64) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// TestRecycleBinSoftDelete 锁定回收站语义（B-06）：
// 删除只置 is_deleted=1 不落物理删除；详情视同不存在；恢复回到正常列表；彻底删除仅限回收站内条目。
func TestRecycleBinSoftDelete(t *testing.T) {
	f := newVarFixture(t)
	a := f.newAPI(t, "待删接口", "/v1/x", "")

	// ① 删除 → 正常列表消失、回收站可见、物理行仍在
	if err := f.s.DeleteApi(f.ctx, f.admin.ID, f.admin.Type, a.ID); err != nil {
		t.Fatalf("DeleteApi: %v", err)
	}
	if containsID(apiIDs(t, f, 0), a.ID) {
		t.Errorf("删除后正常列表不应再出现接口 %d", a.ID)
	}
	if !containsID(apiIDs(t, f, 1), a.ID) {
		t.Errorf("删除后回收站列表应出现接口 %d", a.ID)
	}
	rows, err := f.s.repo.ApisByIDsAny(f.ctx, []uint64{a.ID})
	if err != nil {
		t.Fatalf("ApisByIDsAny: %v", err)
	}
	if len(rows) != 1 || rows[0].IsDeleted != 1 {
		t.Fatalf("软删后应保留物理行且 is_deleted=1, got %+v", rows)
	}

	// ② 详情按「不存在」处理，避免回收站内容被当作正常条目读写
	if _, err := f.s.GetApi(f.ctx, f.admin.ID, f.admin.Type, a.ID); err == nil {
		t.Error("回收站中的接口不应能读到详情")
	}

	// ③ 恢复 → 回到正常列表
	res, err := f.s.RestoreApis(f.ctx, f.admin.ID, f.admin.Type, []uint64{a.ID})
	if err != nil {
		t.Fatalf("RestoreApis: %v", err)
	}
	if res.Restored != 1 {
		t.Errorf("Restored = %d, want 1", res.Restored)
	}
	if !containsID(apiIDs(t, f, 0), a.ID) {
		t.Errorf("恢复后接口 %d 应回到正常列表", a.ID)
	}
	if _, err := f.s.GetApi(f.ctx, f.admin.ID, f.admin.Type, a.ID); err != nil {
		t.Errorf("恢复后应能读到详情: %v", err)
	}

	// ④ 彻底删除只允许回收站内条目：正常条目直接 purge 必须被拒绝
	if _, err := f.s.PurgeApis(f.ctx, f.admin.ID, f.admin.Type, []uint64{a.ID}); err == nil {
		t.Error("未入回收站的接口不应允许彻底删除")
	}

	// ⑤ 入回收站后彻底删除 → 物理行消失
	if err := f.s.DeleteApi(f.ctx, f.admin.ID, f.admin.Type, a.ID); err != nil {
		t.Fatalf("DeleteApi 二次: %v", err)
	}
	pr, err := f.s.PurgeApis(f.ctx, f.admin.ID, f.admin.Type, []uint64{a.ID})
	if err != nil {
		t.Fatalf("PurgeApis: %v", err)
	}
	if pr.Purged != 1 {
		t.Errorf("Purged = %d, want 1", pr.Purged)
	}
	if left, err := f.s.repo.ApisByIDsAny(f.ctx, []uint64{a.ID}); err != nil || len(left) != 0 {
		t.Errorf("彻底删除后不应再有物理行, got %+v err=%v", left, err)
	}
}

// TestRestoreWithDeletedGroupChain 恢复条目时必须把「已软删的祖先分组链」一并还原，
// 否则条目虽然复活，却仍挂在已删目录下不可见。
func TestRestoreWithDeletedGroupChain(t *testing.T) {
	f := newVarFixture(t)
	ctx := f.ctx

	parent, err := f.s.CreateGroup(ctx, f.admin.ID, f.admin.Type, dto.GroupCreateReq{
		ProjectID: f.proj.ID, Title: "父目录", Type: model.GroupTypeAPI,
	})
	if err != nil {
		t.Fatalf("CreateGroup parent: %v", err)
	}
	child, err := f.s.CreateGroup(ctx, f.admin.ID, f.admin.Type, dto.GroupCreateReq{
		ProjectID: f.proj.ID, PID: parent.ID, Title: "子目录", Type: model.GroupTypeAPI,
	})
	if err != nil {
		t.Fatalf("CreateGroup child: %v", err)
	}
	a, err := f.s.CreateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, GroupID: child.ID, ApiName: "目录内接口", URL: "/v1/y", HttpMethodType: "GET",
	})
	if err != nil {
		t.Fatalf("CreateApi: %v", err)
	}

	// 接口与两级目录依次删除：此时条目、子目录、父目录全是 is_deleted=1
	for _, step := range []struct {
		name string
		fn   func() error
	}{
		{"DeleteApi", func() error { return f.s.DeleteApi(ctx, f.admin.ID, f.admin.Type, a.ID) }},
		{"DeleteGroup child", func() error { return f.s.DeleteGroup(ctx, f.admin.ID, f.admin.Type, child.ID) }},
		{"DeleteGroup parent", func() error { return f.s.DeleteGroup(ctx, f.admin.ID, f.admin.Type, parent.ID) }},
	} {
		if err := step.fn(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
	}

	res, err := f.s.RestoreApis(ctx, f.admin.ID, f.admin.Type, []uint64{a.ID})
	if err != nil {
		t.Fatalf("RestoreApis: %v", err)
	}
	if res.Restored != 1 {
		t.Errorf("Restored = %d, want 1", res.Restored)
	}
	// 祖先链共 2 个分组（子 + 父），均需还原
	if res.Groups != 2 {
		t.Errorf("Groups = %d, want 2（子目录与父目录都应还原）", res.Groups)
	}
	for _, id := range []uint64{parent.ID, child.ID} {
		g, err := f.s.repo.GetGroupAny(ctx, id)
		if err != nil {
			t.Fatalf("GetGroupAny(%d): %v", id, err)
		}
		if g.IsDeleted != 0 {
			t.Errorf("分组 %d 应已还原, is_deleted=%d", id, g.IsDeleted)
		}
	}
	// 还原后的目录必须能被列表读到，条目才真正可见
	groups, err := f.s.ListGroups(ctx, f.admin.ID, f.admin.Type, f.proj.ID, model.GroupTypeAPI)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	seen := map[uint64]bool{}
	for _, g := range groups {
		seen[g.ID] = true
	}
	if !seen[child.ID] {
		t.Error("还原后子目录应出现在分组列表")
	}
}

// TestRecycleBinRejectsForeignIDs 批量入口必须拒绝不存在的 id 与超限批量。
func TestRecycleBinRejectsForeignIDs(t *testing.T) {
	f := newVarFixture(t)

	if _, err := f.s.RestoreApis(f.ctx, f.admin.ID, f.admin.Type, []uint64{999999}); err == nil {
		t.Error("恢复不存在的 id 应报错")
	}
	if _, err := f.s.RestoreApis(f.ctx, f.admin.ID, f.admin.Type, nil); err == nil {
		t.Error("空 id 列表应报错")
	}
	tooMany := make([]uint64, batchMax+1)
	for i := range tooMany {
		tooMany[i] = uint64(i + 1)
	}
	if _, err := f.s.PurgeApis(f.ctx, f.admin.ID, f.admin.Type, tooMany); err == nil {
		t.Errorf("超过 %d 条的批量应被拒绝", batchMax)
	}
}
