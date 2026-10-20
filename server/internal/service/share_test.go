package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// TestShareLinkFlow 覆盖分享链接：生成 → 免登录读取 → 过期 → 失效。
func TestShareLinkFlow(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()
	api := f.newAPI(t, "share-api", "https://api/share/list", "")
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeRequest, ProjectID: f.proj.ID, ApiID: api.ID, Name: "token", Value: strPtr("t")})

	link, err := f.s.CreateShareLink(ctx, f.admin.ID, f.admin.Type, dto.ShareLinkReq{
		ProjectID: f.proj.ID, Remark: "给前端", ExpireDays: 7,
	})
	if err != nil {
		t.Fatalf("CreateShareLink: %v", err)
	}
	if len(link.Token) != 32 || link.ExpireTime == nil {
		t.Fatalf("令牌与有效期不符：%+v", link)
	}

	view, err := f.s.SharePayload(ctx, link.Token)
	if err != nil {
		t.Fatalf("SharePayload: %v", err)
	}
	// 注意：分组可选，夹具项目没有分组，这里只校验项目与接口。
	if view.Project.ID != f.proj.ID || len(view.Apis) != 1 || view.Remark != "给前端" {
		t.Fatalf("分享数据不完整：%+v", view)
	}

	// 过期令牌：直接改库，模拟时间流逝。
	past := time.Now().Add(-time.Hour)
	if err := f.s.repo.DB().Model(&model.ShareLink{}).Where("id = ?", link.ID).
		Update("expire_time", past).Error; err != nil {
		t.Fatalf("设置过期时间: %v", err)
	}
	_, err = f.s.SharePayload(ctx, link.Token)
	var be *BizError
	if !errors.As(err, &be) || !strings.Contains(be.Msg, "已过期") {
		t.Fatalf("过期令牌应被拒绝，got %v", err)
	}

	// 失效后不可读。
	if err := f.s.DeleteShareLink(ctx, f.admin.ID, f.admin.Type, link.ID); err != nil {
		t.Fatalf("DeleteShareLink: %v", err)
	}
	if _, err := f.s.SharePayload(ctx, link.Token); err == nil {
		t.Fatal("已删除的分享链接不应可读")
	}
}

// TestMockResponse 覆盖 mock：按方法+路径命中、变量渲染、未命中 404、方法兜底。
func TestMockResponse(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()
	if _, err := f.s.CreateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: "mock-api", HttpMethodType: "GET",
		URL:              "https://api.example.com/mock/users?id=1",
		HttpReturnSample: json.RawMessage(`{"code":200,"data":{"env":"{{mockenv}}","tok":"{{mocksecret}}"}}`),
	}); err != nil {
		t.Fatalf("CreateApi: %v", err)
	}
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, Name: "mockenv", Value: strPtr("mock-dev")})
	// 审核 C-1 回归：/mock 免鉴权，明文 secret 变量不得参与渲染（占位符保留原样）。
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, Name: "mocksecret", Value: strPtr("s3cr3t-token"), IsSecret: 1})

	res, err := f.s.MockResponse(ctx, f.proj.ID, "GET", "/mock/users")
	if err != nil {
		t.Fatalf("MockResponse: %v", err)
	}
	if res.Status != 200 || !strings.Contains(res.Body, "mock-dev") {
		t.Fatalf("应命中样例并渲染变量，got %d %s", res.Status, res.Body)
	}
	if strings.Contains(res.Body, "s3cr3t-token") || !strings.Contains(res.Body, "{{mocksecret}}") {
		t.Fatalf("secret 变量不得在免鉴权 /mock 泄出明文，got %s", res.Body)
	}

	// 方法不同 → 兜底返回同路径接口。
	if res, err = f.s.MockResponse(ctx, f.proj.ID, "POST", "/mock/users"); err != nil || res.Status != 200 {
		t.Fatalf("同路径不同方法应兜底命中，got %v %+v", err, res)
	}

	// 路径未命中 → 404。
	res, err = f.s.MockResponse(ctx, f.proj.ID, "GET", "/mock/absent")
	if err != nil {
		t.Fatalf("MockResponse(未命中): %v", err)
	}
	if res.Status != 404 || !strings.Contains(res.Body, "未匹配到接口") {
		t.Fatalf("未命中应返回 404 提示，got %d %s", res.Status, res.Body)
	}
}

// TestPostmanRoundTrip 覆盖 Postman 导出 → 导入另一个项目 → 重复导入去重。
func TestPostmanRoundTrip(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()
	grp, err := f.s.CreateGroup(ctx, f.admin.ID, f.admin.Type, dto.GroupCreateReq{
		ProjectID: f.proj.ID, Title: "用户", Type: model.GroupTypeAPI,
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if _, err := f.s.CreateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, GroupID: grp.ID, ApiName: "列表", HttpMethodType: "POST",
		URL:               "https://api.example.com/user/list?page=1",
		HttpRequestHeader: json.RawMessage(`[{"name":"X-A","value":"1"}]`),
		HttpRequestParams: json.RawMessage(`[{"fieldName":"name","type":"string","example":"tzq"}]`),
		HttpReturnSample:  json.RawMessage(`{"code":200}`),
	}); err != nil {
		t.Fatalf("CreateApi: %v", err)
	}

	col, err := f.s.ExportPostman(ctx, f.proj.ID, 0)
	if err != nil {
		t.Fatalf("ExportPostman: %v", err)
	}
	items, _ := col["item"].([]map[string]any)
	if len(items) != 1 || items[0]["name"] != "用户" {
		t.Fatalf("分组应导出为文件夹，got %+v", items)
	}

	target, err := f.s.CreateProject(ctx, f.admin.ID, dto.ProjectCreateReq{Title: "pm-target"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	raw, err := json.Marshal(col)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	res, err := f.s.ImportPostman(ctx, f.admin.ID, f.admin.Type, target.ID, "append", raw)
	if err != nil {
		t.Fatalf("ImportPostman: %v", err)
	}
	if res.Imported != 1 {
		t.Fatalf("应导入 1 个接口，got %+v", res)
	}
	apis, err := f.s.repo.ListApisByProject(ctx, target.ID)
	if err != nil {
		t.Fatalf("ListApisByProject: %v", err)
	}
	if len(apis) != 1 || apis[0].Source != SourcePostman {
		t.Fatalf("导入结果不符，got %+v", apis)
	}
	if !strings.Contains(apis[0].HttpRequestParams, "page") || !strings.Contains(strings.ToLower(apis[0].HttpRequestHeader), "x-a") {
		t.Fatalf("查询参数与请求头应被解析，got params=%s headers=%s", apis[0].HttpRequestParams, apis[0].HttpRequestHeader)
	}

	// 重复导入：按方法 + URL 去重。
	again, err := f.s.ImportPostman(ctx, f.admin.ID, f.admin.Type, target.ID, "append", raw)
	if err != nil {
		t.Fatalf("ImportPostman(重复): %v", err)
	}
	if again.Imported != 0 || again.Skipped == 0 {
		t.Fatalf("重复导入应全部跳过，got %+v", again)
	}

	// 非法内容：不合法 JSON 报参数错误。
	_, err = f.s.ImportPostman(ctx, f.admin.ID, f.admin.Type, target.ID, "append", []byte("{oops"))
	var be *BizError
	if !errors.As(err, &be) || be.Code != errcode.CodeParamError {
		t.Fatalf("非法 JSON 应报参数错误，got %v", err)
	}
}
