package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"apidoc/internal/config"
	"apidoc/internal/database"
	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestService 基于独立临时文件 SQLite 构建全链路 service（真实 repo + 迁移）。
func newTestService(t *testing.T) *Service {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{}
	cfg.JWT.Secret = "test-secret"
	cfg.JWT.ExpireHours = 1
	return New(repository.New(db, cfg), cfg)
}

func TestAuthFlow(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)

	if _, err := s.InitAdmin(ctx, "admin", "admin123"); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	res, err := s.Login(ctx, dto.LoginReq{Name: "admin", Pwd: "admin123"})
	if err != nil || res.Token == "" {
		t.Fatalf("Login: %v %+v", err, res)
	}
	if _, err := s.Login(ctx, dto.LoginReq{Name: "admin", Pwd: "bad"}); err == nil {
		t.Fatal("expected login failure for wrong password")
	}

	bob, err := s.Register(ctx, dto.RegisterReq{Name: "bob", Pwd: "p123456", RePwd: "p123456", Email: "b@e.com"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := s.Register(ctx, dto.RegisterReq{Name: "bob", Pwd: "x", RePwd: "x"}); err == nil {
		t.Fatal("expected duplicate register failure")
	}
	if err := s.UpdatePwd(ctx, bob.ID, dto.UpdatePwdReq{OldPwd: "p123456", Pwd: "newpass", RePwd: "newpass"}); err != nil {
		t.Fatalf("UpdatePwd: %v", err)
	}
	if _, err := s.Login(ctx, dto.LoginReq{Name: "bob", Pwd: "p123456"}); err == nil {
		t.Fatal("expected old password rejected after UpdatePwd")
	}
	if _, err := s.Login(ctx, dto.LoginReq{Name: "bob", Pwd: "newpass"}); err != nil {
		t.Fatalf("expected new password accepted: %v", err)
	}
}

func TestProjectPermissionFlow(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	admin, err := s.InitAdmin(ctx, "admin", "admin123")
	if err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	bob, err := s.Register(ctx, dto.RegisterReq{Name: "bob", Pwd: "p123456", RePwd: "p123456"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	proj, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "demo"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	// 负责人由创建者自动获得；bob 起初无权限。
	if perm := s.MyPermission(ctx, bob.ID, bob.Type, proj.ID); perm != 0 {
		t.Fatalf("bob perm want 0, got %d", perm)
	}
	if err := s.AddMember(ctx, admin.ID, admin.Type, dto.MemberAddReq{ProjectID: proj.ID, UserID: bob.ID, Permission: model.PermReadonly}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	// 只读成员不得删除项目。
	err = s.DeleteProject(ctx, bob.ID, bob.Type, proj.ID)
	var be *BizError
	if !errors.As(err, &be) || be.Code != errcode.CodeForbidden {
		t.Fatalf("expected forbidden BizError, got %v", err)
	}
	// 提权后可删除。
	if err := s.SetPermission(ctx, admin.ID, admin.Type, dto.SetPermissionReq{ProjectID: proj.ID, UserID: bob.ID, Permission: model.PermAll}); err != nil {
		t.Fatalf("SetPermission: %v", err)
	}
	if err := s.DeleteProject(ctx, bob.ID, bob.Type, proj.ID); err != nil {
		t.Fatalf("DeleteProject after grant: %v", err)
	}
	// 管理员始终放行。
	if perm := s.MyPermission(ctx, admin.ID, admin.Type, proj.ID); perm != model.PermAll {
		t.Fatalf("admin perm want %d, got %d", model.PermAll, perm)
	}
}

// TestProjectTitleReuse 覆盖名称唯一性的两条边界：
// 未删除项目重名必须被拒；软删除后同名必须可以重建（曾被全局唯一索引阻挡）。
func TestProjectTitleReuse(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	admin, err := s.InitAdmin(ctx, "admin", "admin123")
	if err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}

	first, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "reuse"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "reuse"}); err == nil {
		t.Fatal("expected duplicate title rejected")
	}

	other, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "other"})
	if err != nil {
		t.Fatalf("CreateProject other: %v", err)
	}
	if _, err := s.UpdateProject(ctx, admin.ID, admin.Type, dto.ProjectUpdateReq{ID: other.ID, Title: "reuse"}); err == nil {
		t.Fatal("expected rename to existing title rejected")
	}
	// 未改名时不应被自身名称挡住。
	if _, err := s.UpdateProject(ctx, admin.ID, admin.Type, dto.ProjectUpdateReq{ID: other.ID, Title: "other"}); err != nil {
		t.Fatalf("UpdateProject same title: %v", err)
	}

	if err := s.DeleteProject(ctx, admin.ID, admin.Type, first.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "reuse"}); err != nil {
		t.Fatalf("expected title reusable after soft delete: %v", err)
	}
}

// TestUserLangPreference 界面语言偏好：本人可改、白名单校验、登录响应带回、不可改他人。
func TestUserLangPreference(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	if _, err := s.InitAdmin(ctx, "admin", "admin123"); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	bob, err := s.Register(ctx, dto.RegisterReq{Name: "bob", Pwd: "p123456", RePwd: "p123456"})
	if err != nil {
		t.Fatalf("Register bob: %v", err)
	}
	carol, err := s.Register(ctx, dto.RegisterReq{Name: "carol", Pwd: "p123456", RePwd: "p123456"})
	if err != nil {
		t.Fatalf("Register carol: %v", err)
	}
	if bob.Lang != "" {
		t.Fatalf("新用户 lang 应为空（未选择），got %q", bob.Lang)
	}

	// 本人即可改自己的界面语言，且登录响应（/me 同源）带回。
	if _, err := s.UpdateUser(ctx, *bob, bob.ID, dto.UserUpdateReq{Lang: model.LangEnUS}); err != nil {
		t.Fatalf("本人改语言应放行: %v", err)
	}
	res, err := s.Login(ctx, dto.LoginReq{Name: "bob", Pwd: "p123456"})
	if err != nil || res.User.Lang != model.LangEnUS {
		t.Fatalf("登录响应应带回 lang: %v %+v", err, res.User)
	}

	// 非管理员改他人语言 → 403。
	var be *BizError
	_, err = s.UpdateUser(ctx, *bob, carol.ID, dto.UserUpdateReq{Lang: model.LangEnUS})
	if !errors.As(err, &be) || be.Code != errcode.CodeForbidden {
		t.Fatalf("改他人语言应 403，got %v", err)
	}

	// 白名单外的语言 → 参数错误，且不改动已存值。
	_, err = s.UpdateUser(ctx, *bob, bob.ID, dto.UserUpdateReq{Lang: "fr-FR"})
	if !errors.As(err, &be) || be.Code != errcode.CodeParamError {
		t.Fatalf("非法语言应参数错误，got %v", err)
	}
	after, err := s.GetUser(ctx, bob.ID)
	if err != nil || after.Lang != model.LangEnUS {
		t.Fatalf("非法请求不应改动已存语言: %v %q", err, after.Lang)
	}
}
