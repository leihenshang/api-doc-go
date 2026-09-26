package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// varFixture 变量测试夹具：管理员 + 项目 + 两个环境（dev 为默认 / test）。
type varFixture struct {
	s     *Service
	ctx   context.Context
	admin *model.UserInfo
	proj  *model.Project
	dev   *model.VariableEnv
	test  *model.VariableEnv
}

func newVarFixture(t *testing.T) *varFixture {
	t.Helper()
	ctx := context.Background()
	s := newTestService(t)
	admin, err := s.InitAdmin(ctx, "admin", "admin123")
	if err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	proj, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "varproj"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	dev, err := s.CreateVariableEnv(ctx, admin.ID, admin.Type, dto.VariableEnvReq{ProjectID: proj.ID, Name: "dev"})
	if err != nil {
		t.Fatalf("CreateVariableEnv dev: %v", err)
	}
	if dev.IsDefault != model.VarEnvDefault {
		t.Fatalf("首个环境应自动成为默认环境，got is_default=%d", dev.IsDefault)
	}
	testEnv, err := s.CreateVariableEnv(ctx, admin.ID, admin.Type, dto.VariableEnvReq{ProjectID: proj.ID, Name: "test"})
	if err != nil {
		t.Fatalf("CreateVariableEnv test: %v", err)
	}
	return &varFixture{s: s, ctx: ctx, admin: admin, proj: proj, dev: dev, test: testEnv}
}

// newAPI 建一个接口（headers 为请求头 JSON 文本，可选）。
func (f *varFixture) newAPI(t *testing.T, name, apiURL string, headers string) *model.Api {
	t.Helper()
	a, err := f.s.CreateApi(f.ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: name, URL: apiURL, HttpMethodType: "GET",
		HttpRequestHeader: json.RawMessage(headers),
	})
	if err != nil {
		t.Fatalf("CreateApi: %v", err)
	}
	return a
}

// newVar 以管理员身份建变量。
func (f *varFixture) newVar(t *testing.T, req dto.VariableReq) *VarView {
	t.Helper()
	v, err := f.s.CreateVariable(f.ctx, f.admin.ID, f.admin.Type, req)
	if err != nil {
		t.Fatalf("CreateVariable %s: %v", req.Name, err)
	}
	return v
}

// registerReadonly 注册只读成员并加入项目。
func (f *varFixture) registerReadonly(t *testing.T, name string) *model.UserInfo {
	t.Helper()
	bob, err := f.s.Register(f.ctx, dto.RegisterReq{Name: name, Pwd: "p123456", RePwd: "p123456"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := f.s.AddMember(f.ctx, f.admin.ID, f.admin.Type, dto.MemberAddReq{
		ProjectID: f.proj.ID, UserID: bob.ID, Permission: model.PermReadonly,
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	return bob
}

func strPtr(s string) *string { return &s }

// resolveValue 取解析结果中某个变量的值与来源。
func resolveValue(items []ResolveItem, name string) (string, string, bool) {
	for _, it := range items {
		if it.Name == name {
			return it.Value, it.Source, true
		}
	}
	return "", "", false
}

// TestVariableResolvePrecedence 覆盖四层优先级与环境隔离：
// 请求级 > 项目级(当前环境) > 项目级(通用) > 全局；绑定其它环境的变量不参与解析。
func TestVariableResolvePrecedence(t *testing.T) {
	f := newVarFixture(t)
	api := f.newAPI(t, "list", "https://{{host}}/api/user/list?token={{token}}", "")

	f.newVar(t, dto.VariableReq{Scope: model.VarScopeGlobal, Name: "token", Value: strPtr("global")})
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, Name: "token", Value: strPtr("common")})
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID, Name: "token", Value: strPtr("dev")})
	reqVar := f.newVar(t, dto.VariableReq{Scope: model.VarScopeRequest, ProjectID: f.proj.ID, ApiID: api.ID, Name: "token", Value: strPtr("request")})
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, Name: "host", Value: strPtr("common-host")})
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.test.ID, Name: "host", Value: strPtr("test-host")})

	// dev 环境：token 取请求级，host 回落通用。
	items, err := f.s.ResolveVariables(f.ctx, f.proj.ID, api.ID, f.dev.ID)
	if err != nil {
		t.Fatalf("ResolveVariables: %v", err)
	}
	if v, src, ok := resolveValue(items, "token"); !ok || v != "request" || src != varSourceRequest {
		t.Fatalf("dev 环境 token 应取请求级，got %q/%q ok=%v", v, src, ok)
	}
	if v, src, _ := resolveValue(items, "host"); v != "common-host" || src != varSourceProject {
		t.Fatalf("dev 环境 host 应回落通用，got %q/%q", v, src)
	}

	// 删除请求级后：token 取项目级(dev)。
	if err := f.s.DeleteVariable(f.ctx, f.admin.ID, f.admin.Type, reqVar.ID); err != nil {
		t.Fatalf("DeleteVariable: %v", err)
	}
	items, err = f.s.ResolveVariables(f.ctx, f.proj.ID, api.ID, f.dev.ID)
	if err != nil {
		t.Fatalf("ResolveVariables: %v", err)
	}
	if v, _, _ := resolveValue(items, "token"); v != "dev" {
		t.Fatalf("dev 环境 token 应为 dev，got %q", v)
	}

	// test 环境：token 回落到通用，host 取 test，dev 专属值不参与。
	items, err = f.s.ResolveVariables(f.ctx, f.proj.ID, api.ID, f.test.ID)
	if err != nil {
		t.Fatalf("ResolveVariables: %v", err)
	}
	if v, _, _ := resolveValue(items, "token"); v != "common" {
		t.Fatalf("test 环境 token 应为通用值 common，got %q", v)
	}
	if v, _, _ := resolveValue(items, "host"); v != "test-host" {
		t.Fatalf("test 环境 host 应为 test-host，got %q", v)
	}
}

// TestVariableSecretMasking 覆盖敏感值脱敏：列表与解析只回掩码，仅具备写权限者 with_value 可取明文。
func TestVariableSecretMasking(t *testing.T) {
	f := newVarFixture(t)
	api := f.newAPI(t, "secret", "https://api/{{apikey}}", "")
	f.newVar(t, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: f.proj.ID, Name: "apikey",
		Value: strPtr("s3cret"), IsSecret: model.VarSecret,
	})

	q := dto.VariableListQuery{ProjectID: f.proj.ID, Scope: model.VarScopeProject}
	views, err := f.s.ListVariables(f.ctx, f.admin.ID, f.admin.Type, q, false)
	if err != nil {
		t.Fatalf("ListVariables: %v", err)
	}
	if len(views) != 1 || views[0].Value != varMaskedValue || !views[0].HasValue {
		t.Fatalf("列表应回掩码且标记有值，got %+v", views)
	}

	items, err := f.s.ResolveVariables(f.ctx, f.proj.ID, api.ID, f.dev.ID)
	if err != nil {
		t.Fatalf("ResolveVariables: %v", err)
	}
	if v, _, _ := resolveValue(items, "apikey"); v != varMaskedValue {
		t.Fatalf("解析结果应回掩码，got %q", v)
	}

	// 管理员 with_value=1 可取明文（用于编辑回显）。
	plain, err := f.s.ListVariables(f.ctx, f.admin.ID, f.admin.Type, q, true)
	if err != nil {
		t.Fatalf("ListVariables with_value: %v", err)
	}
	if len(plain) != 1 || plain[0].Value != "s3cret" {
		t.Fatalf("管理员 with_value 应回明文，got %+v", plain)
	}

	// 只读成员即便带 with_value 也只能拿到掩码。
	bob := f.registerReadonly(t, "bob")
	readonly, err := f.s.ListVariables(f.ctx, bob.ID, bob.Type, q, true)
	if err != nil {
		t.Fatalf("ListVariables readonly: %v", err)
	}
	if len(readonly) != 1 || readonly[0].Value != varMaskedValue {
		t.Fatalf("只读成员不应取得明文，got %+v", readonly)
	}
}

// TestVariableEnvDeleteGuard 覆盖环境删除拦截：默认环境、仍被变量引用的环境都拒绝并给出明细。
func TestVariableEnvDeleteGuard(t *testing.T) {
	f := newVarFixture(t)

	// dev 是默认环境 → 拒绝。
	err := f.s.DeleteVariableEnv(f.ctx, f.admin.ID, f.admin.Type, f.dev.ID)
	var be *BizError
	if !errors.As(err, &be) || be.Code != errcode.CodeFailed || !strings.Contains(be.Msg, "默认环境") {
		t.Fatalf("删除默认环境应被拒绝并提示先改默认，got %v", err)
	}

	// 把默认切到 test 后，dev 仍被变量引用 → 拒绝并列出变量名。
	if err := f.s.SetDefaultVariableEnv(f.ctx, f.admin.ID, f.admin.Type, f.test.ID); err != nil {
		t.Fatalf("SetDefaultVariableEnv: %v", err)
	}
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID, Name: "baseUrl", Value: strPtr("https://dev")})
	tokenVar := f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID, Name: "token", Value: strPtr("t")})

	err = f.s.DeleteVariableEnv(f.ctx, f.admin.ID, f.admin.Type, f.dev.ID)
	if !errors.As(err, &be) || !strings.Contains(be.Msg, "baseUrl") || !strings.Contains(be.Msg, "token") {
		t.Fatalf("被引用的环境应拒绝删除并列出变量名，got %v", err)
	}

	refs, err := f.s.VariableEnvRefs(f.ctx, f.dev.ID)
	if err != nil {
		t.Fatalf("VariableEnvRefs: %v", err)
	}
	if !refs.Blocked || refs.Env.Name != "dev" || len(refs.Variables) != 2 {
		t.Fatalf("引用预检结果不符，got %+v", refs)
	}

	// 变量清掉后即可删除。
	if err := f.s.DeleteVariable(f.ctx, f.admin.ID, f.admin.Type, tokenVar.ID); err != nil {
		t.Fatalf("DeleteVariable: %v", err)
	}
	vars, err := f.s.ListVariables(f.ctx, f.admin.ID, f.admin.Type, dto.VariableListQuery{ProjectID: f.proj.ID, Scope: model.VarScopeProject, EnvID: &f.dev.ID}, false)
	if err != nil {
		t.Fatalf("ListVariables by env: %v", err)
	}
	for _, v := range vars {
		if err := f.s.DeleteVariable(f.ctx, f.admin.ID, f.admin.Type, v.ID); err != nil {
			t.Fatalf("DeleteVariable: %v", err)
		}
	}
	if err := f.s.DeleteVariableEnv(f.ctx, f.admin.ID, f.admin.Type, f.dev.ID); err != nil {
		t.Fatalf("DeleteVariableEnv after cleanup: %v", err)
	}
	envs, err := f.s.ListVariableEnvs(f.ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("ListVariableEnvs: %v", err)
	}
	if len(envs) != 1 || envs[0].Name != "test" {
		t.Fatalf("删除后应只剩 test，got %+v", envs)
	}

	// 唯一的（默认）环境允许删除：项目回到无环境状态。
	if err := f.s.DeleteVariableEnv(f.ctx, f.admin.ID, f.admin.Type, f.test.ID); err != nil {
		t.Fatalf("删除唯一的默认环境应放行: %v", err)
	}
	envs, err = f.s.ListVariableEnvs(f.ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("ListVariableEnvs: %v", err)
	}
	if len(envs) != 0 {
		t.Fatalf("删除后应无环境，got %+v", envs)
	}
}

// TestVariableUniquenessAndReuse 覆盖同层同环境唯一、跨环境同名允许、软删除后可重建，
// 并校验 AutoMigrate 建出的是带 WHERE 的局部唯一索引。
func TestVariableUniquenessAndReuse(t *testing.T) {
	f := newVarFixture(t)

	var ddl string
	if err := f.s.repo.DB().Raw("SELECT sql FROM sqlite_master WHERE name = 'uk_variable_key'").Scan(&ddl).Error; err != nil {
		t.Fatalf("查询索引定义: %v", err)
	}
	if !strings.Contains(ddl, "WHERE") {
		t.Fatalf("uk_variable_key 应为排除软删除行的局部唯一索引，实际 DDL: %q", ddl)
	}

	first := f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID, Name: "baseUrl", Value: strPtr("a")})

	// 同层同环境重名被拒。
	_, err := f.s.CreateVariable(f.ctx, f.admin.ID, f.admin.Type, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID, Name: "baseUrl",
	})
	var be *BizError
	if !errors.As(err, &be) || be.Code != errcode.CodeFailed {
		t.Fatalf("同层同环境重名应被拒，got %v", err)
	}

	// 不同环境同名允许。
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.test.ID, Name: "baseUrl", Value: strPtr("b")})

	// 名称非法被拒。
	_, err = f.s.CreateVariable(f.ctx, f.admin.ID, f.admin.Type, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: f.proj.ID, Name: "1-bad",
	})
	if !errors.As(err, &be) || be.Code != errcode.CodeParamError {
		t.Fatalf("非法变量名应返回参数错误，got %v", err)
	}

	// 软删除后同名可重建（局部唯一索引的核心价值）。
	if err := f.s.DeleteVariable(f.ctx, f.admin.ID, f.admin.Type, first.ID); err != nil {
		t.Fatalf("DeleteVariable: %v", err)
	}
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID, Name: "baseUrl", Value: strPtr("c")})
}

// TestVariablePermissions 覆盖写权限：只读成员不能建项目/请求变量，全局变量仅管理员可维护。
func TestVariablePermissions(t *testing.T) {
	f := newVarFixture(t)
	bob := f.registerReadonly(t, "bob")
	api := f.newAPI(t, "list", "https://api/user/list", "")

	_, err := f.s.CreateVariable(f.ctx, bob.ID, bob.Type, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: f.proj.ID, Name: "baseUrl", Value: strPtr("x"),
	})
	var be *BizError
	if !errors.As(err, &be) || be.Code != errcode.CodeForbidden {
		t.Fatalf("只读成员建项目变量应被拒，got %v", err)
	}

	_, err = f.s.CreateVariable(f.ctx, bob.ID, bob.Type, dto.VariableReq{
		Scope: model.VarScopeRequest, ProjectID: f.proj.ID, ApiID: api.ID, Name: "token", Value: strPtr("x"),
	})
	if !errors.As(err, &be) || be.Code != errcode.CodeForbidden {
		t.Fatalf("只读成员建请求变量应被拒，got %v", err)
	}

	_, err = f.s.CreateVariable(f.ctx, bob.ID, bob.Type, dto.VariableReq{
		Scope: model.VarScopeGlobal, Name: "apikey", Value: strPtr("x"),
	})
	if !errors.As(err, &be) || be.Code != errcode.CodeForbidden {
		t.Fatalf("非管理员建全局变量应被拒，got %v", err)
	}

	// 管理员可建全局变量；提权后成员可建项目变量。
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeGlobal, Name: "apikey", Value: strPtr("x")})
	if err := f.s.SetPermission(f.ctx, f.admin.ID, f.admin.Type, dto.SetPermissionReq{
		ProjectID: f.proj.ID, UserID: bob.ID, Permission: model.PermAll,
	}); err != nil {
		t.Fatalf("SetPermission: %v", err)
	}
	if _, err := f.s.CreateVariable(f.ctx, bob.ID, bob.Type, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: f.proj.ID, Name: "baseUrl", Value: strPtr("x"),
	}); err != nil {
		t.Fatalf("提权后应可建项目变量: %v", err)
	}
}

// TestVariableSummaryMatchesResolve 覆盖列表徽标统计与解析口径一致（含切环境后未定义数变化）。
func TestVariableSummaryMatchesResolve(t *testing.T) {
	f := newVarFixture(t)
	headers := `[{"name":"Authorization","value":"Bearer {{token}}"},{"name":"X-Trace-Id","value":"{{traceId}}"}]`
	api := f.newAPI(t, "list", "https://{{host}}/api/user/list?name={{keyword}}", headers)

	f.newVar(t, dto.VariableReq{Scope: model.VarScopeRequest, ProjectID: f.proj.ID, ApiID: api.ID, Name: "token", Value: strPtr("t")})
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID, Name: "host", Value: strPtr("https://dev")})
	f.newVar(t, dto.VariableReq{Scope: model.VarScopeGlobal, Name: "keyword", Value: strPtr("tzq")})

	sum, err := f.s.SummarizeVariables(f.ctx, f.proj.ID, f.dev.ID, []uint64{api.ID})
	if err != nil {
		t.Fatalf("SummarizeVariables: %v", err)
	}
	got := sum[api.ID]
	if got.RequestCount != 1 || len(got.Defined) != 1 || got.Defined[0] != "token" {
		t.Fatalf("请求变量统计不符，got %+v", got)
	}
	if got.UndefinedCount != 1 || got.Undefined[0] != "traceId" {
		t.Fatalf("未定义变量统计不符，got %+v", got)
	}

	// 与解析口径一致：定义的变量都在解析表里，未定义的不在。
	items, err := f.s.ResolveVariables(f.ctx, f.proj.ID, api.ID, f.dev.ID)
	if err != nil {
		t.Fatalf("ResolveVariables: %v", err)
	}
	for _, name := range []string{"token", "host", "keyword"} {
		if _, _, ok := resolveValue(items, name); !ok {
			t.Fatalf("解析表缺少已定义变量 %s：%+v", name, items)
		}
	}
	if _, _, ok := resolveValue(items, "traceId"); ok {
		t.Fatal("解析表不应包含未定义变量 traceId")
	}

	// 切到 test 环境：host 绑定在 dev 上 → 变为未定义。
	sum, err = f.s.SummarizeVariables(f.ctx, f.proj.ID, f.test.ID, []uint64{api.ID})
	if err != nil {
		t.Fatalf("SummarizeVariables test env: %v", err)
	}
	got = sum[api.ID]
	foundHost := false
	for _, name := range got.Undefined {
		if name == "host" {
			foundHost = true
		}
	}
	if !foundHost {
		t.Fatalf("test 环境下 host 应为未定义，got %+v", got)
	}
	if _, _, ok := resolveValue(mustResolve(t, f, api.ID, f.test.ID), "host"); ok {
		t.Fatal("test 环境下解析表不应包含 host")
	}

	// api_ids 为空 / 超上限时的参数校验。
	if _, err := f.s.SummarizeVariables(f.ctx, f.proj.ID, f.dev.ID, nil); err == nil {
		t.Fatal("空 api_ids 应报参数错误")
	}
	tooMany := make([]uint64, maxSummaryApiIDs+1)
	for i := range tooMany {
		tooMany[i] = uint64(i + 1)
	}
	if _, err := f.s.SummarizeVariables(f.ctx, f.proj.ID, f.dev.ID, tooMany); err == nil {
		t.Fatal("超过上限的 api_ids 应报参数错误")
	}
}

// mustResolve 解析失败直接终止测试。
func mustResolve(t *testing.T, f *varFixture, apiID, envID uint64) []ResolveItem {
	t.Helper()
	items, err := f.s.ResolveVariables(f.ctx, f.proj.ID, apiID, envID)
	if err != nil {
		t.Fatalf("ResolveVariables: %v", err)
	}
	return items
}
