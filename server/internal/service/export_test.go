package service

// 审核 H-17：OpenAPI 导出必须包含全部接口，不能固定 PageSize=200 静默截断。

import (
	"fmt"
	"strings"
	"testing"

	"apidoc/internal/model"
)

// TestExportSkipsDisabledRows 停用行（enabled:false）不进代码片段 / 请求体 / OpenAPI / 契约：
// 它们与客户端发送时的跳过口径一致，缺键仍按启用处理。
func TestExportSkipsDisabledRows(t *testing.T) {
	t.Parallel()
	const raw = `[{"fieldName":"a","example":"1"},{"fieldName":"b","example":"2","enabled":false}]`

	if params := openAPIParams(raw); len(params) != 1 || params[0].Name != "a" {
		t.Fatalf("OpenAPI 参数应只剩启用行, got %+v", params)
	}
	if body := paramsBody(raw); strings.Contains(body, `"b"`) {
		t.Fatalf("请求体不应包含停用参数: %s", body)
	}
	if rows := parseRows(`[{"name":"H","enabled":false}]`); len(rows) != 0 {
		t.Fatalf("停用请求头应被丢弃, got %+v", rows)
	}

	a := &model.Api{
		HttpRequestParams: `[{"fieldName":"a","type":"string"},{"fieldName":"b","type":"string","enabled":false}]`,
		HttpRequestHeader: `[{"name":"X-Off","enabled":false}]`,
	}
	if got := currentContractParams(a); len(got) != 1 || got["a"].Name != "a" {
		t.Fatalf("契约只应包含启用行, got %+v", got)
	}
}

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
