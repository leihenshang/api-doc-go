package service

import (
	"context"
	"encoding/json"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

// pushDoc 提交一条 doc upsert（超管直过权限），返回服务端结果。
func pushDoc(t *testing.T, s *Service, userID, projectID uint64, uid, opID string, p SyncPayload) SyncOpOut {
	t.Helper()
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	resp, err := s.SyncPush(context.Background(), userID, model.UserTypeSuperuser, &SyncPushReq{
		ProjectID:  projectID,
		DeviceID:   "dev-test",
		DeviceName: "测试设备",
		Ops:        []SyncOpIn{{OpID: opID, Op: model.SyncOpUpsert, Type: model.SyncTypeDoc, UID: uid, Payload: body}},
	})
	if err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("push 结果数量异常: %+v", resp.Results)
	}
	return resp.Results[0]
}

// TestSyncDocGroupPlacement 文档的 group_uid 必须落到 doc.group_id（J11）：
// ① 带 group_uid → 命中分组；② 不带 → 未分组；③ 回传载荷也要带 GroupUID（拉取方向）。
func TestSyncDocGroupPlacement(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()

	// 建一个分组，拿它的 ext_uid 作为文档归属
	g, err := f.s.CreateGroup(ctx, f.admin.ID, f.admin.Type, dto.GroupCreateReq{
		ProjectID: f.proj.ID, Title: "目录", Type: model.GroupTypeDoc,
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	// ① 带 group_uid
	out := pushDoc(t, f.s, f.admin.ID, f.proj.ID, "doc-in-group", "op-doc-g1", SyncPayload{
		Name: "分组内文档", Content: "正文", GroupUID: g.ExtUID,
	})
	if out.Status != "ok" {
		t.Fatalf("带 group_uid 的文档应推送成功: %+v", out)
	}
	d, err := f.s.repo.FindDocByExtUID(ctx, f.proj.ID, "doc-in-group")
	if err != nil || d == nil {
		t.Fatalf("文档应已落库: %v", err)
	}
	if d.GroupID != g.ID {
		t.Errorf("文档 group_id = %d, want %d（按 group_uid 落位失败）", d.GroupID, g.ID)
	}

	// ② 不带 group_uid → 未分组
	out = pushDoc(t, f.s, f.admin.ID, f.proj.ID, "doc-no-group", "op-doc-g2", SyncPayload{Name: "根级文档", Content: "x"})
	if out.Status != "ok" {
		t.Fatalf("不带 group_uid 的文档应推送成功: %+v", out)
	}
	d2, err := f.s.repo.FindDocByExtUID(ctx, f.proj.ID, "doc-no-group")
	if err != nil || d2 == nil {
		t.Fatalf("文档应已落库: %v", err)
	}
	if d2.GroupID != 0 {
		t.Errorf("不带 group_uid 的文档 group_id 应为 0，got %d", d2.GroupID)
	}

	// ③ 回传载荷带 GroupUID（客户端拉取时按它落位）
	p := f.s.docSyncPayload(ctx, f.proj.ID, "doc-in-group")
	if p.GroupUID != g.ExtUID {
		t.Errorf("回传 GroupUID = %q, want %q", p.GroupUID, g.ExtUID)
	}
}
