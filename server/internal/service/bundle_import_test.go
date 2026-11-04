package service

// U3：导入 .apidocgo.json 集合包（目录结构 + 请求富字段 + 文档）。

import (
	"strings"
	"testing"

	"apidoc/internal/model"
	"apidoc/internal/pkg/page"
)

const sampleBundle = `{
  "format": "api-doc-go/collection",
  "version": 1,
  "collection": {"name": "demo", "uid": "c1"},
  "folders": [
    {"path": "api", "name": "api", "seq": 1},
    {"path": "api/user", "name": "user", "seq": 2}
  ],
  "requests": [
    {
      "uid": "r1", "name": "登录", "path": "api/user/登录.yml",
      "method": "POST", "url": "{{host}}/login",
      "summary": "登录接口", "docs": "# 说明",
      "params": [{"name": "q", "value": "1", "enabled": true}],
      "headers": [{"name": "X-A", "value": "1", "enabled": true}],
      "body": {"type": "json", "raw": "{\"a\":1}"},
      "auth": {"type": "bearer", "token": "t"},
      "settings": {"timeout": 30},
      "script": {"preRequest": "// x"},
      "varsPreRequest": [{"name": "v", "value": "1", "enabled": true}],
      "asserts": [{"expr": "status == 200"}],
      "respDoc": {"type": "object", "params": [{"fieldName": "id", "type": "integer"}], "sample": "{}", "contentType": "json"},
      "status": 3, "version": "v2"
    }
  ],
  "docs": [
    {"uid": "d1", "name": "说明", "path": "api/user/说明.md", "content": "# 正文", "icon": "📘", "seq": 5}
  ]
}`

func TestImportBundle(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	res, err := f.s.ImportBundle(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, "append", []byte(sampleBundle))
	if err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	if res.Imported != 4 || res.Skipped != 0 {
		t.Fatalf("应导入 4 条（2 目录 + 1 请求 + 1 文档），got %+v", res)
	}

	// 目录层级：api/user 的 PID 应为 api 分组
	groups, err := f.s.repo.ListGroups(ctx, f.proj.ID, model.GroupTypeAPI)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	var rootID, childID uint64
	for _, g := range groups {
		switch g.Title {
		case "api":
			rootID = g.ID
		case "user":
			childID = g.ID
			if g.PID != rootID && rootID != 0 {
				t.Fatalf("user 分组应挂在 api 下：pid=%d root=%d", g.PID, rootID)
			}
		}
	}
	if rootID == 0 || childID == 0 {
		t.Fatalf("两级目录都应建出来: %+v", groups)
	}

	// 请求：富字段原样落列
	pq := page.Query{PageNum: 1, PageSize: 100}
	apis, _, err := f.s.repo.ListApis(ctx, f.proj.ID, 0, 0, 0, "", pq)
	if err != nil {
		t.Fatalf("ListApis: %v", err)
	}
	var a *model.Api
	for i := range apis {
		if apis[i].URL == "{{host}}/login" {
			a = &apis[i]
		}
	}
	if a == nil {
		t.Fatalf("请求未导入: %+v", apis)
	}
	if a.HttpMethodType != "POST" || a.ApiName != "登录" || a.GroupID != childID {
		t.Fatalf("请求基础字段不符: %+v", a)
	}
	if a.HttpRequestSummary != "登录接口" || a.Description != "# 说明" {
		t.Fatalf("概述/说明未落库: %+v", a)
	}
	if !strings.Contains(a.HttpRequestHeader, "X-A") || !strings.Contains(a.HttpRequestParams, "\"q\"") {
		t.Fatalf("请求头/参数未落库: header=%q params=%q", a.HttpRequestHeader, a.HttpRequestParams)
	}
	if a.HttpRequestBodyType != "json" || a.HttpRequestBody != `{"a":1}` {
		t.Fatalf("请求体不符: type=%q body=%q", a.HttpRequestBodyType, a.HttpRequestBody)
	}
	if !strings.Contains(a.HttpAuth, "bearer") || a.RequestSettings == "" || a.HttpScript == "" {
		t.Fatalf("认证/设置/脚本未落库: %+v", a)
	}
	if a.ScriptVars == "" || a.ScriptAsserts == "" {
		t.Fatalf("前置变量/断言未落库: vars=%q asserts=%q", a.ScriptVars, a.ScriptAsserts)
	}
	if a.HttpReturnType != "object" || a.HttpReturnContentType != "json" || a.HttpReturnSample != "{}" {
		t.Fatalf("响应文档未落库: %+v", a)
	}
	if a.Status != 3 || a.Version != "v2" {
		t.Fatalf("生命周期未落库: status=%d version=%q", a.Status, a.Version)
	}

	// 文档
	docs, _, err := f.s.repo.ListDocs(ctx, f.proj.ID, 0, "", 0, pq)
	if err != nil {
		t.Fatalf("ListDocs: %v", err)
	}
	if len(docs) != 1 || docs[0].Title != "说明" || docs[0].Content != "# 正文" {
		t.Fatalf("文档未导入: %+v", docs)
	}
	if docs[0].GroupID != childID || docs[0].Priority != 5 {
		t.Fatalf("文档归属/顺序不符: %+v", docs[0])
	}

	// 二次导入：目录复用、请求按方法+URL 去重 → 只 skipped=1，不会重复长目录
	res2, err := f.s.ImportBundle(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, "append", []byte(sampleBundle))
	if err != nil {
		t.Fatalf("ImportBundle 第二次: %v", err)
	}
	if res2.Skipped != 1 {
		t.Fatalf("重复请求应跳过 1 条，got %+v", res2)
	}
	groups2, err := f.s.repo.ListGroups(ctx, f.proj.ID, model.GroupTypeAPI)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups2) != len(groups) {
		t.Fatalf("重复导入不应新建目录：before=%d after=%d", len(groups), len(groups2))
	}
}

func TestImportBundleRejectsBadPayload(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	if _, err := f.s.ImportBundle(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, "append", []byte("not json")); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
	if _, err := f.s.ImportBundle(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, "append",
		[]byte(`{"format":"postman","version":1}`)); err == nil {
		t.Fatal("format 不符应报错")
	}
	if _, err := f.s.ImportBundle(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, "append",
		[]byte(`{"format":"api-doc-go/collection","version":99}`)); err == nil {
		t.Fatal("高版本应报错")
	}
}
