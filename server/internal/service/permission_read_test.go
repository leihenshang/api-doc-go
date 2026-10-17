package service

import (
	"context"
	"errors"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/page"
)

// TestReadAPIsRequireProjectPermission 覆盖 P0-1：非项目成员的读请求必须被拒
// （GetApi / ListApis / GetDoc / ListDocs / ListGroups / ListFieldMappings）。
func TestReadAPIsRequireProjectPermission(t *testing.T) {
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
