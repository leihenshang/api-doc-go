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
	t.Parallel()
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

// TestSyncDocNameRoundTrip 文档名双向兼容：
// ① 新客户端只发 name（历史写法）→ 服务端要落到 title；
// ② 旧客户端只发 title → 也要落到 title；
// ③ 回传载荷同时带 name 与 title（客户端历史读 name）。
func TestSyncDocNameRoundTrip(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()

	out := pushDoc(t, f.s, f.admin.ID, f.proj.ID, "doc-by-name", "op-doc-name", SyncPayload{
		Name: "项目说明", Content: "x",
	})
	if out.Status != "ok" {
		t.Fatalf("只带 name 的文档应推送成功: %+v", out)
	}
	d, err := f.s.repo.FindDocByExtUID(ctx, f.proj.ID, "doc-by-name")
	if err != nil || d == nil {
		t.Fatalf("文档应已落库: %v", err)
	}
	if d.Title != "项目说明" {
		t.Errorf("doc.title = %q，payload.Name 应落到 title", d.Title)
	}

	out = pushDoc(t, f.s, f.admin.ID, f.proj.ID, "doc-by-title", "op-doc-title", SyncPayload{
		Title: "只给标题", Content: "x",
	})
	if out.Status != "ok" {
		t.Fatalf("只带 title 的文档应推送成功: %+v", out)
	}
	d2, err := f.s.repo.FindDocByExtUID(ctx, f.proj.ID, "doc-by-title")
	if err != nil || d2 == nil {
		t.Fatalf("文档应已落库: %v", err)
	}
	if d2.Title != "只给标题" {
		t.Errorf("doc.title = %q，payload.Title 应作为兜底", d2.Title)
	}

	p := f.s.docSyncPayload(ctx, f.proj.ID, "doc-by-name")
	if p.Name != "项目说明" || p.Title != "项目说明" {
		t.Errorf("回传载荷 name=%q title=%q，应两者都带（兼容按 name 读的客户端）", p.Name, p.Title)
	}
}

// TestCreateDocReturnsChangeID Web 新建文档的返回体要带本次 revision：
// DB 已由 SetLastChange 更新，但内存对象不赋值时返回 0，调用方据此算 base_rev 会失真。
func TestCreateDocReturnsChangeID(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	d, err := f.s.CreateDoc(context.Background(), f.admin.ID, f.admin.Type, dto.DocCreateReq{
		ProjectID: f.proj.ID, Title: "带版本号", Content: "x",
	})
	if err != nil {
		t.Fatalf("CreateDoc: %v", err)
	}
	if d.LastChangeID <= 0 {
		t.Errorf("返回体 last_change_id = %d，应带本次变更流水 id", d.LastChangeID)
	}
}
