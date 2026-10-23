package service

// 审核 H-17：OpenAPI 导出必须包含全部接口，不能固定 PageSize=200 静默截断。

import (
	"fmt"
	"testing"

	"apidoc/internal/model"
)

func TestOpenAPIExportBeyondPageSize(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	const total = 205 // 原实现 PageSize=200，会静默丢掉最后 5 条

	for i := 0; i < total; i++ {
		a := &model.Api{
			ProjectID:      f.proj.ID,
			ApiName:        fmt.Sprintf("api-%03d", i),
			URL:            fmt.Sprintf("/export/%03d", i),
			HttpMethodType: "GET",
		}
		if err := f.s.repo.CreateApi(f.ctx, a); err != nil {
			t.Fatalf("CreateApi #%d: %v", i, err)
		}
	}

	doc, err := f.s.OpenAPIExport(f.ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0)
	if err != nil {
		t.Fatalf("OpenAPIExport: %v", err)
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("导出结果缺少 paths: %#v", doc)
	}
	if len(paths) != total {
		t.Fatalf("导出接口数 = %d，want %d（不能按 200 截断）", len(paths), total)
	}
}
