package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/page"
)

// TestReadAPIsRequireProjectPermission 覆盖 P0-1：非项目成员的读请求必须被拒
// （GetApi / ListApis / GetDoc / ListDocs / ListGroups / ListFieldMappings）。
func TestReadAPIsRequireProjectPermission(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()

	// 项目内的接口与文档（由管理员创建）
	api, err := f.s.CreateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, GroupID: 0, ApiName: "ping", HttpMethodType: "GET", URL: "/ping",
	})
	if err != nil {
		t.Fatalf("CreateApi: %v", err)
	}
	doc, err := f.s.CreateDoc(ctx, f.admin.ID, f.admin.Type, dto.DocCreateReq{
		ProjectID: f.proj.ID, Title: "说明",
	})
	if err != nil {
		t.Fatalf("CreateDoc: %v", err)
	}

	// 非成员用户（普通用户，未被加入该项目）
	outsider := &model.UserInfo{
		Name: "outsider", Pwd: "x", Type: model.UserTypeNormal, State: model.UserStateNormal,
	}
	if err := f.s.repo.CreateUser(ctx, outsider); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	assertForbidden := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s：非成员应被拒绝，却成功了", name)
		}
		var be *BizError
		if !errors.As(err, &be) {
			t.Fatalf("%s：期望业务错误（403），得到 %v", name, err)
		}
	}

	_, err = f.s.GetApi(ctx, outsider.ID, outsider.Type, api.ID)
	assertForbidden("GetApi", err)

	_, err = f.s.ListApis(ctx, outsider.ID, outsider.Type, dto.ApiListQuery{ProjectID: f.proj.ID})
	assertForbidden("ListApis", err)

	_, err = f.s.GetDoc(ctx, outsider.ID, outsider.Type, doc.ID)
	assertForbidden("GetDoc", err)

	_, err = f.s.ListDocs(ctx, outsider.ID, outsider.Type, f.proj.ID, 0, "", 0, page.Query{PageNum: 1, PageSize: 20})
	assertForbidden("ListDocs", err)

	_, err = f.s.ListGroups(ctx, outsider.ID, outsider.Type, f.proj.ID, -1)
	assertForbidden("ListGroups", err)

	_, err = f.s.ListFieldMappings(ctx, outsider.ID, outsider.Type, f.proj.ID)
	assertForbidden("ListFieldMappings", err)

	// 反向确认：项目管理员仍可正常读
	if _, err := f.s.GetApi(ctx, f.admin.ID, f.admin.Type, api.ID); err != nil {
		t.Fatalf("管理员读接口应成功: %v", err)
	}
	if _, err := f.s.ListDocs(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, "", 0, page.Query{PageNum: 1, PageSize: 20}); err != nil {
		t.Fatalf("管理员列文档应成功: %v", err)
	}
}

// TestAuditC2ReadEndpointsRequirePermission 覆盖审核 C-2：12 个曾无鉴权的读/轻写端点，
// 非成员一律 403——防止「修复边界之外的遗漏」继续复发（对照 permission 边界清单逐点断言）。
func TestAuditC2ReadEndpointsRequirePermission(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()

	api, err := f.s.CreateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: "c2-api", HttpMethodType: "GET", URL: "/c2",
		HttpReturnSample: json.RawMessage(`{"code":200}`),
	})
	if err != nil {
		t.Fatalf("CreateApi: %v", err)
	}
	doc, err := f.s.CreateDoc(ctx, f.admin.ID, f.admin.Type, dto.DocCreateReq{ProjectID: f.proj.ID, Title: "c2-doc"})
	if err != nil {
		t.Fatalf("CreateDoc: %v", err)
	}
	bug, err := f.s.CreateBug(ctx, f.admin.ID, f.admin.Type, dto.BugCreateReq{ProjectID: f.proj.ID, Title: "b", Content: "c"})
	if err != nil {
		t.Fatalf("CreateBug: %v", err)
	}
	outsider := &model.UserInfo{Name: "c2-outsider", Pwd: "x", Type: model.UserTypeNormal, State: model.UserStateNormal}
	if err := f.s.repo.CreateUser(ctx, outsider); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	o := outsider.ID
	ot := outsider.Type

	cases := []struct {
		name string
		call func() error
	}{
		{"ExportPostman", func() error { _, e := f.s.ExportPostman(ctx, o, ot, f.proj.ID, 0); return e }},
		{"OpenAPIExport", func() error { _, e := f.s.OpenAPIExport(ctx, o, ot, f.proj.ID, 0); return e }},
		{"GetProject", func() error { _, e := f.s.GetProject(ctx, o, ot, f.proj.ID); return e }},
		{"ListMembers", func() error { _, e := f.s.ListMembers(ctx, o, ot, f.proj.ID); return e }},
		{"ListApiSamples", func() error { _, e := f.s.ListApiSamples(ctx, o, ot, api.ID); return e }},
		{"ListBugs", func() error {
			_, e := f.s.ListBugs(ctx, o, ot, f.proj.ID, 0, page.Query{PageNum: 1, PageSize: 10})
			return e
		}},
		{"GetBug", func() error { _, e := f.s.GetBug(ctx, o, ot, bug.ID); return e }},
		{"ListBugAssign", func() error { _, e := f.s.ListBugAssign(ctx, o, ot, bug.ID); return e }},
		{"ListLogs", func() error {
			_, e := f.s.ListLogs(ctx, o, ot, f.proj.ID, 0, 0, page.Query{PageNum: 1, PageSize: 10})
			return e
		}},
		{"ResolveVariables", func() error { _, e := f.s.ResolveVariables(ctx, o, ot, f.proj.ID, api.ID, 0); return e }},
		{"SummarizeVariables", func() error { _, e := f.s.SummarizeVariables(ctx, o, ot, f.proj.ID, 0, []uint64{api.ID}); return e }},
		{"VariableEnvRefs", func() error { _, e := f.s.VariableEnvRefs(ctx, o, ot, f.dev.ID); return e }},
		{"LikeDoc", func() error { return f.s.LikeDoc(ctx, o, ot, doc.ID) }},
	}
	for _, c := range cases {
		err := c.call()
		if err == nil {
			t.Fatalf("%s：非成员应被拒绝，却成功了", c.name)
		}
		var be *BizError
		if !errors.As(err, &be) {
			t.Fatalf("%s：期望业务权限错误，得到 %v", c.name, err)
		}
	}

	// 反向确认：成员（管理员）全部正常放行（抽样三个代表路径）。
	if _, err := f.s.ExportPostman(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0); err != nil {
		t.Fatalf("管理员导出 Postman 应成功: %v", err)
	}
	if _, err := f.s.GetProject(ctx, f.admin.ID, f.admin.Type, f.proj.ID); err != nil {
		t.Fatalf("管理员读项目详情应成功: %v", err)
	}
	if err := f.s.LikeDoc(ctx, f.admin.ID, f.admin.Type, doc.ID); err != nil {
		t.Fatalf("成员点赞应成功: %v", err)
	}
}
