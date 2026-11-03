package service

// R3：接口生命周期元数据（草稿/已发布/已废弃 + 版本 + 废弃时间）的同步语义：
//   - 服务端权威：下发载荷带 api_status / version / deprecated_at；
//   - 缺键不改：客户端回推的载荷不带这三个键时，服务端已设的状态不被抹掉。

import (
	"encoding/json"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

func TestSyncLifecycleStatusIsServerOwned(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	// Web 端建接口并标记为「已废弃」+ 版本 v2。
	a, err := f.s.CreateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: "待废弃接口", URL: "/life", HttpMethodType: "GET",
		Status: model.ApiStatusDeprecated, Version: "v2",
	})
	if err != nil {
		t.Fatalf("CreateApi: %v", err)
	}

	// ① 下发载荷必须带生命周期元数据。
	raw, err := f.s.loadPayload(ctx, f.proj.ID, model.SyncTypeAPI, a.ExtUID, true)
	if err != nil {
		t.Fatalf("loadPayload: %v", err)
	}
	var p SyncPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("载荷反序列化: %v", err)
	}
	if p.ApiStatus != int(model.ApiStatusDeprecated) || p.Version != "v2" {
		t.Fatalf("载荷应带 api_status=3/version=v2，实际 %+v", p)
	}

	// ② 客户端回推（不带状态键）→ 服务端状态保持不变（不能被同步抹掉）。
	body, _ := json.Marshal(SyncPayload{Name: "客户端改的名", Method: "GET", URL: "/life"})
	if _, err := f.s.SyncPush(ctx, f.admin.ID, f.admin.Type, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev-test",
		Ops: []SyncOpIn{{OpID: "op-life-1", Op: model.SyncOpUpsert, Type: model.SyncTypeAPI, UID: a.ExtUID, Payload: body}},
	}); err != nil {
		t.Fatalf("SyncPush 客户端回推: %v", err)
	}
	got, err := f.s.repo.FindApiByExtUID(ctx, f.proj.ID, a.ExtUID)
	if err != nil {
		t.Fatalf("查询接口: %v", err)
	}
	if got.Status != model.ApiStatusDeprecated || got.Version != "v2" {
		t.Fatalf("客户端回推不应改动服务端状态，实际 status=%d version=%q", got.Status, got.Version)
	}
	if got.ApiName != "客户端改的名" {
		t.Fatalf("客户端回推的名称未生效: %q", got.ApiName)
	}

	// ③ 带状态键的载荷可以改状态（供将来带状态的管理端使用）。
	body2, _ := json.Marshal(SyncPayload{Name: "客户端改的名", Method: "GET", URL: "/life", ApiStatus: int(model.ApiStatusDraft)})
	if _, err := f.s.SyncPush(ctx, f.admin.ID, f.admin.Type, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev-test",
		Ops: []SyncOpIn{{OpID: "op-life-2", Op: model.SyncOpUpsert, Type: model.SyncTypeAPI, UID: a.ExtUID, Payload: body2}},
	}); err != nil {
		t.Fatalf("SyncPush 带状态: %v", err)
	}
	got, err = f.s.repo.FindApiByExtUID(ctx, f.proj.ID, a.ExtUID)
	if err != nil {
		t.Fatalf("查询接口: %v", err)
	}
	if got.Status != model.ApiStatusDraft {
		t.Fatalf("带状态键的载荷应落库，实际 status=%d", got.Status)
	}
}

// R5：文档同层排序权重（客户端 seq ↔ 服务端 priority）双向往返。
func TestSyncDocSeqRoundTrip(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	d, err := f.s.CreateDoc(ctx, f.admin.ID, f.admin.Type, dto.DocCreateReq{
		ProjectID: f.proj.ID, Title: "排序文档", Content: "正文",
	})
	if err != nil {
		t.Fatalf("CreateDoc: %v", err)
	}

	// 客户端推 seq=7 → 服务端落 priority=7。
	body, _ := json.Marshal(SyncPayload{Name: "排序文档", Title: "排序文档", Content: "正文", DocSeq: 7})
	if _, err := f.s.SyncPush(ctx, f.admin.ID, f.admin.Type, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev-test",
		Ops: []SyncOpIn{{OpID: "op-seq-1", Op: model.SyncOpUpsert, Type: model.SyncTypeDoc, UID: d.ExtUID, Payload: body}},
	}); err != nil {
		t.Fatalf("SyncPush doc: %v", err)
	}
	got, err := f.s.repo.FindDocByExtUID(ctx, f.proj.ID, d.ExtUID)
	if err != nil {
		t.Fatalf("查询文档: %v", err)
	}
	if got.Priority != 7 {
		t.Fatalf("推送的 seq 应落 priority，实际 %d", got.Priority)
	}

	// 下发载荷带回 doc_seq=7。
	raw, err := f.s.loadPayload(ctx, f.proj.ID, model.SyncTypeDoc, d.ExtUID, true)
	if err != nil {
		t.Fatalf("loadPayload: %v", err)
	}
	var p SyncPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("载荷反序列化: %v", err)
	}
	if p.DocSeq != 7 {
		t.Fatalf("下发载荷应带 doc_seq=7，实际 %d", p.DocSeq)
	}
}
