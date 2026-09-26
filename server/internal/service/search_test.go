package service

import (
	"context"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

// TestGlobalSearch 覆盖全局搜索的可见性与命中范围：
// 普通用户只能搜到自己参与的项目，管理员可搜全量；支持接口名/URL、文档标题/正文；空关键词返回空。
func TestGlobalSearch(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	admin, err := s.InitAdmin(ctx, "admin", "admin123")
	if err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	pa, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "search-A"})
	if err != nil {
		t.Fatalf("CreateProject A: %v", err)
	}
	pb, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "search-B"})
	if err != nil {
		t.Fatalf("CreateProject B: %v", err)
	}

	if _, err := s.CreateApi(ctx, admin.ID, admin.Type, dto.ApiCreateReq{
		ProjectID: pa.ID, ApiName: "zeta-api", URL: "https://api/zeta", HttpMethodType: "GET",
	}); err != nil {
		t.Fatalf("CreateApi A: %v", err)
	}
	if _, err := s.CreateDoc(ctx, admin.ID, admin.Type, dto.DocCreateReq{
		ProjectID: pa.ID, Title: "zeta-doc", Content: "正文里也提到 zeta 这个词",
	}); err != nil {
		t.Fatalf("CreateDoc A: %v", err)
	}
	if _, err := s.CreateApi(ctx, admin.ID, admin.Type, dto.ApiCreateReq{
		ProjectID: pb.ID, ApiName: "zeta-api-b", URL: "https://api/zeta", HttpMethodType: "GET",
	}); err != nil {
		t.Fatalf("CreateApi B: %v", err)
	}

	bob, err := s.Register(ctx, dto.RegisterReq{Name: "bob", Pwd: "p123456", RePwd: "p123456"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := s.AddMember(ctx, admin.ID, admin.Type, dto.MemberAddReq{
		ProjectID: pa.ID, UserID: bob.ID, Permission: model.PermReadonly,
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	// 只读成员：只能命中参与项目 A。
	res, err := s.Search(ctx, bob.ID, bob.Type, "zeta", 10)
	if err != nil {
		t.Fatalf("Search(bob): %v", err)
	}
	if len(res.Apis) != 1 || res.Apis[0].ProjectID != pa.ID {
		t.Fatalf("bob 应只搜到项目 A 的接口，got %+v", res.Apis)
	}
	if len(res.Docs) != 1 || res.Docs[0].ProjectID != pa.ID {
		t.Fatalf("bob 应只搜到项目 A 的文档，got %+v", res.Docs)
	}

	// 管理员：两个项目都能搜到。
	resAdmin, err := s.Search(ctx, admin.ID, admin.Type, "zeta", 10)
	if err != nil {
		t.Fatalf("Search(admin): %v", err)
	}
	if len(resAdmin.Apis) != 2 {
		t.Fatalf("admin 应搜到两个项目的接口，got %+v", resAdmin.Apis)
	}

	// 文档正文命中（非标题）。
	resBody, err := s.Search(ctx, bob.ID, bob.Type, "正文里", 10)
	if err != nil {
		t.Fatalf("Search(正文): %v", err)
	}
	if len(resBody.Docs) != 1 {
		t.Fatalf("应命中文档正文，got %+v", resBody.Docs)
	}

	// 空关键词 → 空结果且不报错。
	resEmpty, err := s.Search(ctx, bob.ID, bob.Type, "   ", 10)
	if err != nil {
		t.Fatalf("Search(空): %v", err)
	}
	if len(resEmpty.Apis)+len(resEmpty.Docs)+len(resEmpty.Projects) != 0 {
		t.Fatalf("空关键词应返回空结果，got %+v", resEmpty)
	}
}
