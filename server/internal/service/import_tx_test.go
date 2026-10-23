package service

import (
	"encoding/json"
	"strings"
	"testing"

	"apidoc/internal/model"
)

// openapiDoc 拼一份最小可用的 OpenAPI 文档。
// 每个参数形如 `/path|method|摘要` 或 `/path|method|摘要|tag`。
func openapiDoc(specs ...string) string {
	paths := make([]string, 0, len(specs))
	for _, spec := range specs {
		fields := strings.SplitN(spec, "|", 4)
		path, method, summary := fields[0], fields[1], fields[2]
		tagJSON := ""
		if len(fields) > 3 && fields[3] != "" {
			tagJSON = `,"tags":["` + fields[3] + `"]`
		}
		paths = append(paths, `"`+path+`":{"`+method+`":{"summary":"`+summary+`"`+tagJSON+`}}`)
	}
	return `{"openapi":"3.0.0","info":{"title":"导入","version":"1"},"paths":{` + strings.Join(paths, ",") + `}}`
}

// TestImportOverwriteKeepsDataOnBadBody 覆盖导入会先清空项目现有接口：
// 文档解析失败时必须原样保留旧数据，否则用户一次误操作就丢掉整个项目。
func TestImportOverwriteKeepsDataOnBadBody(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	f.newAPI(t, "老接口", "/legacy/keep", "")

	// 非法 body + 覆盖策略：报参数错误，但一条接口都不能少。
	if _, err := f.s.ImportOpenAPI(f.ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, StrategyOverwrite, []byte("{oops")); err == nil {
		t.Fatal("非法 OpenAPI 应报错")
	}
	apis, err := f.s.repo.ListApisByProject(f.ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("ListApisByProject: %v", err)
	}
	if len(apis) != 1 || apis[0].URL != "/legacy/keep" {
		t.Fatalf("解析失败不应动旧数据，got %+v", apis)
	}

	// Postman 同理：body 非法时报错且不动数据。
	if _, err := f.s.ImportPostman(f.ctx, f.admin.ID, f.admin.Type, f.proj.ID, StrategyOverwrite, []byte("{oops")); err == nil {
		t.Fatal("非法 Postman 集合应报错")
	}
	if apis, err = f.s.repo.ListApisByProject(f.ctx, f.proj.ID); err != nil || len(apis) != 1 {
		t.Fatalf("解析失败不应动旧数据，got %+v (%v)", apis, err)
	}
}

// TestImportOverwriteReplacesApis 覆盖策略成功后旧接口应被清空（软删除），只留新导入内容。
func TestImportOverwriteReplacesApis(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	f.newAPI(t, "老接口", "/legacy/gone", "")

	res, err := f.s.ImportOpenAPI(f.ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, StrategyOverwrite,
		[]byte(openapiDoc("/v1/order|get|订单详情")))
	if err != nil {
		t.Fatalf("ImportOpenAPI: %v", err)
	}
	if res.Imported != 1 {
		t.Fatalf("应导入 1 条，got %+v", res)
	}
	apis, err := f.s.repo.ListApisByProject(f.ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("ListApisByProject: %v", err)
	}
	if len(apis) != 1 || apis[0].URL != "/v1/order" {
		t.Fatalf("覆盖导入后应只剩新接口，got %+v", apis)
	}
}

// TestImportSharedTagReusesGroup 同一份文档里重复出现的 tag 只能建一个分组。
func TestImportSharedTagReusesGroup(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	body := []byte(openapiDoc(
		"/v1/a|get|A|用户",
		"/v1/b|get|B|用户",
		"/v1/c|post|C|用户",
	))
	if _, err := f.s.ImportOpenAPI(f.ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, StrategyAppend, body); err != nil {
		t.Fatalf("ImportOpenAPI: %v", err)
	}
	groups, err := f.s.repo.ListGroups(f.ctx, f.proj.ID, model.GroupTypeAPI)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	titled := make([]model.Group, 0, len(groups))
	for _, g := range groups {
		if g.Title == "用户" {
			titled = append(titled, g)
		}
	}
	if len(titled) != 1 {
		t.Fatalf("同名 tag 应复用同一个分组，got %d 个: %+v", len(titled), titled)
	}
	apis, err := f.s.repo.ListApisByProject(f.ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("ListApisByProject: %v", err)
	}
	if len(apis) != 3 {
		t.Fatalf("应导入 3 条接口，got %d", len(apis))
	}
	for _, a := range apis {
		if a.GroupID != titled[0].ID {
			t.Fatalf("接口应落在复用分组下，got group_id=%d want %d", a.GroupID, titled[0].ID)
		}
	}
}

// TestImportSkipsGhostMethods Path Item 的 summary / parameters 之类不能变成接口。
func TestImportSkipsGhostMethods(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	body := []byte(`{
	  "openapi": "3.0.0",
	  "info": {"title": "t", "version": "1"},
	  "paths": {
	    "/v1/user": {
	      "summary": "用户资源",
	      "description": "整段说明",
	      "parameters": [{"name": "tenant", "in": "header", "required": true}],
	      "get": {"summary": "查询用户"}
	    }
	  }
	}`)
	res, err := f.s.ImportOpenAPI(f.ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, StrategyAppend, body)
	if err != nil {
		t.Fatalf("ImportOpenAPI: %v", err)
	}
	if res.Imported != 1 {
		t.Fatalf("只应导入 1 条接口，got %+v", res)
	}
	apis, err := f.s.repo.ListApisByProject(f.ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("ListApisByProject: %v", err)
	}
	if len(apis) != 1 {
		t.Fatalf("不应出现幽灵接口，got %+v", apis)
	}
	if apis[0].HttpMethodType != "GET" || apis[0].URL != "/v1/user" {
		t.Fatalf("接口字段异常: %+v", apis[0])
	}
}

// TestImportOpenAPICarriesParamsAndSample 导入必须把参数、请求头、响应样例一起落库。
//
// 回归背景：此前 ImportOpenAPI 只写 api_name / method / url，spec 里的 parameters、
// header、responses.example 全部丢弃——用户看到「导入成功」，打开接口却是三个「暂无」（P0-2）。
func TestImportOpenAPICarriesParamsAndSample(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	spec := []byte(`{
	  "openapi": "3.0.3",
	  "info": {"title": "t", "version": "1"},
	  "paths": {
	    "/v1/orders": {
	      "get": {
	        "summary": "订单列表",
	        "description": "分页返回订单",
	        "parameters": [
	          {"name": "page", "in": "query", "required": false, "description": "页码", "example": 1},
	          {"name": "status", "in": "query", "required": true, "description": "订单状态", "example": "paid"},
	          {"name": "Authorization", "in": "header", "required": true, "description": "令牌", "example": "Bearer t"}
	        ],
	        "responses": {
	          "200": {"description": "ok", "content": {"application/json": {"example": {
	            "code": 200, "data": {"id": 1, "roles": ["admin"]}}}}}
	        }
	      }
	    }
	  }
	}`)
	if _, err := f.s.ImportOpenAPI(f.ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, StrategyAppend, spec); err != nil {
		t.Fatalf("ImportOpenAPI: %v", err)
	}
	apis, err := f.s.repo.ListApisByProject(f.ctx, f.proj.ID)
	if err != nil || len(apis) != 1 {
		t.Fatalf("应导入 1 条，got %d (%v)", len(apis), err)
	}
	a := apis[0]

	if a.Description != "分页返回订单" {
		t.Fatalf("description 未导入: %q", a.Description)
	}

	var params []struct {
		FieldName   string `json:"fieldName"`
		Type        string `json:"type"`
		Required    bool   `json:"required"`
		Description string `json:"description"`
		Example     string `json:"example"`
	}
	if err := json.Unmarshal([]byte(a.HttpRequestParams), &params); err != nil {
		t.Fatalf("http_request_params 不是合法 JSON 数组: %q (%v)", a.HttpRequestParams, err)
	}
	if len(params) != 2 {
		t.Fatalf("query 参数应有 2 个（page/status），got %+v", params)
	}
	if params[0].FieldName != "page" || params[1].FieldName != "status" || !params[1].Required {
		t.Fatalf("参数名/必填未正确映射: %+v", params)
	}
	if params[0].Example != "1" || params[1].Description != "订单状态" {
		t.Fatalf("参数 example/description 未映射: %+v", params)
	}

	var headers []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(a.HttpRequestHeader), &headers); err != nil {
		t.Fatalf("http_request_header 不是合法 JSON 数组: %q (%v)", a.HttpRequestHeader, err)
	}
	if len(headers) != 1 || headers[0].Name != "Authorization" || headers[0].Value != "Bearer t" {
		t.Fatalf("header 参数应单独落到请求头部: %+v", headers)
	}

	if !strings.Contains(a.HttpReturnSample, `"code": 200`) {
		t.Fatalf("成功响应样例未保存: %q", a.HttpReturnSample)
	}
	var ret []struct {
		FieldName string `json:"fieldName"`
		Type      string `json:"type"`
		Example   string `json:"example"`
	}
	if err := json.Unmarshal([]byte(a.HttpReturnParams), &ret); err != nil {
		t.Fatalf("http_return_params 不是合法 JSON 数组: %q (%v)", a.HttpReturnParams, err)
	}
	if len(ret) != 2 || ret[0].FieldName != "code" || ret[1].FieldName != "data" {
		t.Fatalf("返回参数应按样例顶层字段推导并排序: %+v", ret)
	}
	if ret[0].Type != "integer" || ret[1].Type != "object" {
		t.Fatalf("返回参数类型推导有误: %+v", ret)
	}
	if a.HttpReturnType != "object" {
		t.Fatalf("有返回参数时应标记返回类型 object，got %q", a.HttpReturnType)
	}
}
