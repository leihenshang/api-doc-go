package service

// RM-08 契约校验：端点新增/删除、必填变化、类型变化与破坏性计数。

import (
	"encoding/json"
	"testing"

	"apidoc/internal/dto"
)

func TestContractDiff(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)

	mustCreate := func(name, method, path, params, headers string) {
		t.Helper()
		if _, err := f.s.CreateApi(f.ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
			ProjectID: f.proj.ID, ApiName: name, HttpMethodType: method, URL: path,
			HttpRequestParams: json.RawMessage(params), HttpRequestHeader: json.RawMessage(headers),
		}); err != nil {
			t.Fatalf("CreateApi %s: %v", path, err)
		}
	}
	mustCreate("pets", "GET", "/pets",
		`[{"fieldName":"page","type":"int","required":false},{"fieldName":"limit","type":"int","required":false}]`,
		`[{"name":"X-Tenant"}]`)
	mustCreate("old", "DELETE", "/old", "", "")

	spec := []byte(`{
	  "openapi": "3.0.0",
	  "info": {"title": "契约", "version": "1"},
	  "paths": {
	    "/pets": {
	      "get": {
	        "parameters": [
	          {"name": "page", "in": "query", "required": true, "schema": {"type": "integer"}},
	          {"name": "limit", "in": "query", "schema": {"type": "string"}},
	          {"name": "name", "in": "query", "required": true},
	          {"name": "X-Tenant", "in": "header", "required": true}
	        ]
	      }
	    },
	    "/new": {
	      "post": {
	        "parameters": [{"name": "name", "in": "query", "required": true}],
	        "responses": {"200": {"description": "ok"}}
	      }
	    }
	  }
	}`)
	res, err := f.s.ContractDiff(f.ctx, f.admin.ID, f.admin.Type, dto.ContractDiffReq{
		ProjectID: f.proj.ID, OpenAPI: spec,
	})
	if err != nil {
		t.Fatalf("ContractDiff: %v", err)
	}
	if len(res.Added) != 1 || res.Added[0].Method != "POST" || res.Added[0].Path != "/new" {
		t.Fatalf("新增端点异常: %+v", res.Added)
	}
	if len(res.Removed) != 1 || res.Removed[0].Method != "DELETE" || res.Removed[0].Path != "/old" {
		t.Fatalf("删除端点异常: %+v", res.Removed)
	}
	kinds := map[string]dto.ContractChange{}
	for _, c := range res.Changes {
		kinds[c.Kind+":"+c.Param] = c
	}
	if c, ok := kinds["required_changed:page"]; !ok || !c.Breaking {
		t.Fatalf("page 由可选变必填应标记 breaking: %+v", res.Changes)
	}
	if c, ok := kinds["type_changed:limit"]; !ok || !c.Breaking {
		t.Fatalf("limit int→string 应标记 breaking: %+v", res.Changes)
	}
	if c, ok := kinds["param_added:name"]; !ok || !c.Breaking {
		t.Fatalf("新增必填参数应标记 breaking: %+v", res.Changes)
	}
	if _, ok := kinds["required_changed:X-Tenant"]; !ok {
		t.Fatalf("请求头必填变化应被识别: %+v", res.Changes)
	}
	// /old 删除(1) + page 必填(1) + limit 类型(1) + name 新增必填(1) + X-Tenant 必填(1)
	if res.BreakingCount != 5 {
		t.Fatalf("破坏性计数异常: %d, changes=%+v", res.BreakingCount, res.Changes)
	}
}

func TestContractDiffRejectsSwagger(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	_, err := f.s.ContractDiff(f.ctx, f.admin.ID, f.admin.Type, dto.ContractDiffReq{
		ProjectID: f.proj.ID, OpenAPI: []byte(`{"swagger":"2.0","paths":{}}`),
	})
	if err == nil {
		t.Fatal("Swagger 2.0 应返回可读错误")
	}
}
