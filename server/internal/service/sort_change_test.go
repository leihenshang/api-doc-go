package service

// 拖拽排序/移动必须写变更流水：Web 是这类操作的唯一入口，
// 只改 priority / group_id 而不记流水，客户端增量永远拉不到（顺序与归属都不同步）。

import (
	"encoding/json"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

func TestSortNodesRecordsChange(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	d, err := f.s.CreateDoc(ctx, f.admin.ID, f.admin.Type, dto.DocCreateReq{
		ProjectID: f.proj.ID, Title: "排序文档", Content: "正文",
	})
	if err != nil {
		t.Fatalf("CreateDoc: %v", err)
	}
	prevCursor, err := f.s.repo.LatestChangeID(ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("LatestChangeID: %v", err)
	}

	if err := f.s.SortNodes(ctx, f.admin.ID, f.admin.Type, dto.SortReq{
		ProjectID: f.proj.ID, Kind: "doc",
		Items: []dto.SortItem{{ID: d.ID, Priority: 7}},
	}); err != nil {
		t.Fatalf("SortNodes doc: %v", err)
	}

	got, err := f.s.repo.GetDoc(ctx, d.ID)
	if err != nil {
		t.Fatalf("GetDoc: %v", err)
	}
	if got.Priority != 7 {
		t.Fatalf("排序未落库，priority=%d", got.Priority)
	}
	if got.LastChangeID <= d.LastChangeID {
		t.Fatalf("排序应推进文档 revision：before=%d after=%d", d.LastChangeID, got.LastChangeID)
	}
	cur, err := f.s.repo.LatestChangeID(ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("LatestChangeID: %v", err)
	}
	if cur <= prevCursor {
		t.Fatalf("排序应产生变更流水：%d → %d", prevCursor, cur)
	}

	// 增量载荷上能读到新顺序（doc_seq）——客户端据此落 frontmatter 的 seq。
	raw, err := f.s.loadPayload(ctx, f.proj.ID, model.SyncTypeDoc, d.ExtUID, true)
	if err != nil {
		t.Fatalf("loadPayload: %v", err)
	}
	var p SyncPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("载荷反序列化: %v", err)
	}
	if p.DocSeq != 7 {
		t.Fatalf("载荷应带 doc_seq=7，实际 %d", p.DocSeq)
	}
}

func TestSortNodesApiMoveRecordsChange(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	a, err := f.s.CreateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: "待移动接口", URL: "/mv", HttpMethodType: "GET",
	})
	if err != nil {
		t.Fatalf("CreateApi: %v", err)
	}
	prevCursor, err := f.s.repo.LatestChangeID(ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("LatestChangeID: %v", err)
	}
	if err := f.s.SortNodes(ctx, f.admin.ID, f.admin.Type, dto.SortReq{
		ProjectID: f.proj.ID, Kind: "api",
		Items: []dto.SortItem{{ID: a.ID, Priority: 3}},
	}); err != nil {
		t.Fatalf("SortNodes api: %v", err)
	}
	got, err := f.s.repo.GetApi(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetApi: %v", err)
	}
	if got.LastChangeID <= a.LastChangeID {
		t.Fatalf("接口排序/移动应推进 revision：before=%d after=%d", a.LastChangeID, got.LastChangeID)
	}
	cur, err := f.s.repo.LatestChangeID(ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("LatestChangeID: %v", err)
	}
	if cur <= prevCursor {
		t.Fatalf("接口排序应产生变更流水：%d → %d", prevCursor, cur)
	}
}
