package service

// RM-04 接口生命周期：状态过滤、废弃时间维护、版本保留、OpenAPI 导出过滤。

import (
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

func TestApiLifecycle(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)

	published, err := f.s.CreateApi(f.ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: "published", URL: "/life/published", HttpMethodType: "GET",
		Version: "v1",
	})
	if err != nil {
		t.Fatalf("CreateApi published: %v", err)
	}
	if published.Status != model.ApiStatusPublished {
		t.Fatalf("新建接口默认应为已发布，got %d", published.Status)
	}
	draft, err := f.s.CreateApi(f.ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: "draft", URL: "/life/draft", HttpMethodType: "GET",
		Status: model.ApiStatusDraft,
	})
	if err != nil {
		t.Fatalf("CreateApi draft: %v", err)
	}
	deprecated, err := f.s.CreateApi(f.ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: "old", URL: "/life/old", HttpMethodType: "GET",
		Status: model.ApiStatusDeprecated,
	})
	if err != nil {
		t.Fatalf("CreateApi deprecated: %v", err)
	}
	if deprecated.DeprecatedAt == nil {
		t.Fatal("废弃接口应自动记录 deprecated_at")
	}

	// 状态过滤。
	res, err := f.s.ListApis(f.ctx, f.admin.ID, f.admin.Type, dto.ApiListQuery{
		ProjectID: f.proj.ID, Status: model.ApiStatusDraft,
	})
	if err != nil {
		t.Fatalf("ListApis draft: %v", err)
	}
	if res.Total != 1 || len(res.Items) != 1 || res.Items[0].ID != draft.ID {
		t.Fatalf("草稿过滤结果异常: total=%d items=%+v", res.Total, res.Items)
	}

	// 草稿 → 废弃：补时间；废弃 → 发布：清时间。
	updated, err := f.s.UpdateApi(f.ctx, f.admin.ID, f.admin.Type, dto.ApiUpdateReq{
		ID: draft.ID,
		ApiCreateReq: dto.ApiCreateReq{
			ProjectID: f.proj.ID, ApiName: "draft", URL: "/life/draft", HttpMethodType: "GET",
			Status: model.ApiStatusDeprecated,
		},
	})
	if err != nil {
		t.Fatalf("UpdateApi deprecated: %v", err)
	}
	if updated.Status != model.ApiStatusDeprecated || updated.DeprecatedAt == nil {
		t.Fatalf("转为废弃应记录时间: %+v", updated)
	}
	updated, err = f.s.UpdateApi(f.ctx, f.admin.ID, f.admin.Type, dto.ApiUpdateReq{
		ID: draft.ID,
		ApiCreateReq: dto.ApiCreateReq{
			ProjectID: f.proj.ID, ApiName: "draft", URL: "/life/draft", HttpMethodType: "GET",
			Status: model.ApiStatusPublished,
		},
	})
	if err != nil {
		t.Fatalf("UpdateApi published: %v", err)
	}
	if updated.Status != model.ApiStatusPublished || updated.DeprecatedAt != nil {
		t.Fatalf("回到发布应清空废弃时间: %+v", updated)
	}

	// 版本：更新不传时保持原值。
	updated, err = f.s.UpdateApi(f.ctx, f.admin.ID, f.admin.Type, dto.ApiUpdateReq{
		ID: published.ID,
		ApiCreateReq: dto.ApiCreateReq{
			ProjectID: f.proj.ID, ApiName: "published", URL: "/life/published", HttpMethodType: "GET",
		},
	})
	if err != nil {
		t.Fatalf("UpdateApi keep version: %v", err)
	}
	if updated.Version != "v1" {
		t.Fatalf("更新未传 version 应保持原值，got %q", updated.Version)
	}
	draft.Status = model.ApiStatusPublished

	// 导出：草稿/废弃不进 OpenAPI；发布态（含原草稿）保留。
	doc, err := f.s.OpenAPIExport(f.ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0)
	if err != nil {
		t.Fatalf("OpenAPIExport: %v", err)
	}
	paths, _ := doc["paths"].(map[string]any)
	if len(paths) != 2 {
		t.Fatalf("导出应只含 2 个发布态接口，got %d: %+v", len(paths), paths)
	}

	// 非法状态必须拒绝。
	if _, err := f.s.CreateApi(f.ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: "bad", Status: 9,
	}); err == nil {
		t.Fatal("非法状态应返回参数错误")
	}
}
