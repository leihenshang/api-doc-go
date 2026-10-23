package service

// RM-05 快照/diff/回滚：接口保存留历史，diff 能看到字段变化，回滚后内容恢复且再留一份历史。

import (
	"context"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

func TestDocRevisionDiffAndRestore(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	d, err := f.s.CreateDoc(f.ctx, f.admin.ID, f.admin.Type, dto.DocCreateReq{
		ProjectID: f.proj.ID, Title: "旧标题", Content: "旧正文",
	})
	if err != nil {
		t.Fatalf("CreateDoc: %v", err)
	}
	if _, err := f.s.UpdateDoc(f.ctx, f.admin.ID, f.admin.Type, dto.DocUpdateReq{
		ID: d.ID, Title: "新标题", Content: "新正文",
	}); err != nil {
		t.Fatalf("UpdateDoc: %v", err)
	}
	items, err := f.s.ListRevisions(f.ctx, f.admin.ID, f.admin.Type, model.SnapshotEntityDoc, d.ID)
	if err != nil || len(items) != 2 {
		t.Fatalf("文档快照数量异常: %v %+v", err, items)
	}
	diff, err := f.s.RevisionDiff(f.ctx, f.admin.ID, f.admin.Type, model.SnapshotEntityDoc, d.ID, items[1].ID)
	if err != nil {
		t.Fatalf("RevisionDiff doc: %v", err)
	}
	if len(diff.Fields) < 2 {
		t.Fatalf("文档 diff 应含标题/正文: %+v", diff.Fields)
	}
	if err := f.s.RestoreRevision(f.ctx, f.admin.ID, f.admin.Type, model.SnapshotEntityDoc, d.ID, items[1].ID); err != nil {
		t.Fatalf("RestoreRevision doc: %v", err)
	}
	got, err := f.s.GetDoc(f.ctx, f.admin.ID, f.admin.Type, d.ID)
	if err != nil || got.Title != "旧标题" || got.Content != "旧正文" {
		t.Fatalf("文档回滚失败: %+v (%v)", got, err)
	}
}

func TestApiRevisionDiffAndRestore(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	a, err := f.s.CreateApi(f.ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: "旧名称", URL: "/revision", HttpMethodType: "GET",
		Description: "旧说明", Version: "v1",
	})
	if err != nil {
		t.Fatalf("CreateApi: %v", err)
	}
	updated, err := f.s.UpdateApi(f.ctx, f.admin.ID, f.admin.Type, dto.ApiUpdateReq{
		ID: a.ID,
		ApiCreateReq: dto.ApiCreateReq{
			ProjectID: f.proj.ID, ApiName: "新名称", URL: "/revision", HttpMethodType: "POST",
			Description: "新说明", Version: "v2",
		},
	})
	if err != nil {
		t.Fatalf("UpdateApi: %v", err)
	}
	if updated.ApiName != "新名称" {
		t.Fatalf("更新未生效: %+v", updated)
	}

	items, err := f.s.ListRevisions(f.ctx, f.admin.ID, f.admin.Type, model.SnapshotEntityAPI, a.ID)
	if err != nil {
		t.Fatalf("ListRevisions: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("创建 + 更新应各留一份快照，got %d", len(items))
	}
	oldest := items[len(items)-1]
	diff, err := f.s.RevisionDiff(f.ctx, f.admin.ID, f.admin.Type, model.SnapshotEntityAPI, a.ID, oldest.ID)
	if err != nil {
		t.Fatalf("RevisionDiff: %v", err)
	}
	fields := map[string]dto.FieldDiff{}
	for _, d := range diff.Fields {
		fields[d.Field] = d
	}
	for _, key := range []string{"api_name", "http_method_type", "description", "version"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("diff 应包含 %s，got %+v", key, diff.Fields)
		}
	}
	if fields["api_name"].Before != "旧名称" || fields["api_name"].After != "新名称" {
		t.Fatalf("api_name diff 方向错误: %+v", fields["api_name"])
	}

	if err := f.s.RestoreRevision(f.ctx, f.admin.ID, f.admin.Type, model.SnapshotEntityAPI, a.ID, oldest.ID); err != nil {
		t.Fatalf("RestoreRevision: %v", err)
	}
	got, err := f.s.GetApi(f.ctx, f.admin.ID, f.admin.Type, a.ID)
	if err != nil {
		t.Fatalf("GetApi: %v", err)
	}
	if got.ApiName != "旧名称" || got.Description != "旧说明" || got.HttpMethodType != "GET" || got.Version != "v1" {
		t.Fatalf("回滚未恢复历史内容: %+v", got)
	}
	if got.ExtUID != a.ExtUID {
		t.Fatalf("回滚不应丢失 ext_uid: %q -> %q", a.ExtUID, got.ExtUID)
	}

	items, err = f.s.ListRevisions(f.ctx, f.admin.ID, f.admin.Type, model.SnapshotEntityAPI, a.ID)
	if err != nil {
		t.Fatalf("ListRevisions after restore: %v", err)
	}
	if len(items) != 3 || items[0].Action != model.SnapshotActionRestore {
		t.Fatalf("回滚应再留一份 restore 快照: %+v", items)
	}

	// 类型非法必须拒绝。
	if _, err := f.s.ListRevisions(context.Background(), f.admin.ID, f.admin.Type, "bad", a.ID); err == nil {
		t.Fatal("非法快照类型应报参数错误")
	}
}
