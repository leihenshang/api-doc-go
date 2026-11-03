package service

// R1：客户端同步写入（sync push）也要留历史快照，
// 否则 Web 端「历史修订 / 字段级 diff / 回滚」看不到客户端的改动，
// 回滚到旧版本时会把客户端的改动一起吞掉。

import (
	"encoding/json"
	"testing"

	"apidoc/internal/model"
)

func TestSyncPushWritesRevisionSnapshot(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	const uid = "api-snap-uid-1"
	body, _ := json.Marshal(SyncPayload{
		Name: "客户端接口", Method: "GET", URL: "/snap",
		Extra: `{"headers":"","params":""}`,
	})
	resp, err := f.s.SyncPush(ctx, f.admin.ID, f.admin.Type, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev-test", DeviceName: "测试设备",
		Ops: []SyncOpIn{{OpID: "op-snap-1", Op: model.SyncOpUpsert, Type: model.SyncTypeAPI, UID: uid, Payload: body}},
	})
	if err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Status != "ok" {
		t.Fatalf("push 结果异常: %+v", resp.Results)
	}

	api, err := f.s.repo.FindApiByExtUID(ctx, f.proj.ID, uid)
	if err != nil || api == nil {
		t.Fatalf("push 后应能按 ext_uid 查到接口: %v", err)
	}

	// ① 同步写入必须留下一份快照，动作是 sync。
	items, err := f.s.ListRevisions(ctx, f.admin.ID, f.admin.Type, model.SnapshotEntityAPI, api.ID)
	if err != nil {
		t.Fatalf("ListRevisions: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("同步写入应留 1 份快照，got %d", len(items))
	}
	if items[0].Action != model.SnapshotActionSync {
		t.Fatalf("快照动作应为 sync，实际 %q", items[0].Action)
	}

	// ② 再推一次（改名）→ 快照累积到 2 份，且能回滚回旧名字。
	body2, _ := json.Marshal(SyncPayload{Name: "改名后", Method: "GET", URL: "/snap"})
	if _, err := f.s.SyncPush(ctx, f.admin.ID, f.admin.Type, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev-test",
		Ops: []SyncOpIn{{OpID: "op-snap-2", Op: model.SyncOpUpsert, Type: model.SyncTypeAPI, UID: uid, Payload: body2}},
	}); err != nil {
		t.Fatalf("SyncPush 第二次: %v", err)
	}
	items, err = f.s.ListRevisions(ctx, f.admin.ID, f.admin.Type, model.SnapshotEntityAPI, api.ID)
	if err != nil || len(items) != 2 {
		t.Fatalf("两次同步应留 2 份快照: %v %+v", err, items)
	}

	// ③ 回滚到最早那份 → 接口名恢复成第一次推送的名字（证明快照 payload 可用于回滚）。
	oldest := items[len(items)-1]
	if err := f.s.RestoreRevision(ctx, f.admin.ID, f.admin.Type, model.SnapshotEntityAPI, api.ID, oldest.ID); err != nil {
		t.Fatalf("RestoreRevision: %v", err)
	}
	got, err := f.s.repo.FindApiByExtUID(ctx, f.proj.ID, uid)
	if err != nil {
		t.Fatalf("回滚后查询: %v", err)
	}
	if got.ApiName != "客户端接口" {
		t.Fatalf("回滚后应恢复为首次同步的接口名，实际 %q", got.ApiName)
	}
}

func TestSyncPushDocWritesRevisionSnapshot(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	const uid = "doc-snap-uid-1"
	body, _ := json.Marshal(SyncPayload{Name: "客户端文档", Title: "客户端文档", Content: "# 正文"})
	if _, err := f.s.SyncPush(ctx, f.admin.ID, f.admin.Type, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev-test",
		Ops: []SyncOpIn{{OpID: "op-doc-snap-1", Op: model.SyncOpUpsert, Type: model.SyncTypeDoc, UID: uid, Payload: body}},
	}); err != nil {
		t.Fatalf("SyncPush doc: %v", err)
	}
	doc, err := f.s.repo.FindDocByExtUID(ctx, f.proj.ID, uid)
	if err != nil || doc == nil {
		t.Fatalf("push 后应能按 ext_uid 查到文档: %v", err)
	}
	items, err := f.s.ListRevisions(ctx, f.admin.ID, f.admin.Type, model.SnapshotEntityDoc, doc.ID)
	if err != nil {
		t.Fatalf("ListRevisions doc: %v", err)
	}
	if len(items) != 1 || items[0].Action != model.SnapshotActionSync {
		t.Fatalf("文档同步写入应留 1 份 sync 快照: %+v", items)
	}
}
