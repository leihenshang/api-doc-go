package service

import (
	"context"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/page"
)

// newUserFixture 建一个超管 + 一个管理员 + 一个普通用户，用于角色越权测试。
func newUserFixture(t *testing.T) (s *Service, ctx context.Context, admin, manager, plain *model.UserInfo) {
	t.Helper()
	ctx = context.Background()
	s = newTestService(t)
	admin, err := s.InitAdmin(ctx, "root", "root123456")
	if err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	manager, err = s.Register(ctx, dto.RegisterReq{Name: "mgr", Pwd: "p123456", RePwd: "p123456", Email: "m@e.com"})
	if err != nil {
		t.Fatalf("Register mgr: %v", err)
	}
	if _, err := s.UpdateUser(ctx, *admin, manager.ID, dto.UserUpdateReq{Type: model.UserTypeAdmin}); err != nil {
		t.Fatalf("超管提升管理员应成功: %v", err)
	}
	manager.Type = model.UserTypeAdmin

	plain, err = s.Register(ctx, dto.RegisterReq{Name: "bob", Pwd: "p123456", RePwd: "p123456", Email: "b@e.com"})
	if err != nil {
		t.Fatalf("Register bob: %v", err)
	}
	return s, ctx, admin, manager, plain
}

// TestRoleChangeGuards 锁定 W-05 的三条越权边界。
func TestRoleChangeGuards(t *testing.T) {
	t.Parallel()
	s, ctx, admin, manager, plain := newUserFixture(t)

	// ① 普通用户不能给自己提权
	if _, err := s.UpdateUser(ctx, *plain, plain.ID, dto.UserUpdateReq{Type: model.UserTypeSuperuser}); err == nil {
		t.Error("普通用户不应能把自己改成超管")
	}
	// ② 普通用户不能改自己的状态（自我禁用/解禁）
	if _, err := s.UpdateUser(ctx, *plain, plain.ID, dto.UserUpdateReq{State: model.UserStateDisabled}); err == nil {
		t.Error("普通用户不应能改自己的状态")
	}
	// ③ 管理员不能调整角色（admin → 超管 是一条提权链）
	if _, err := s.UpdateUser(ctx, *manager, manager.ID, dto.UserUpdateReq{Type: model.UserTypeSuperuser}); err == nil {
		t.Error("管理员不应能把自己提成超管")
	}
	if _, err := s.UpdateUser(ctx, *manager, plain.ID, dto.UserUpdateReq{Type: model.UserTypeAdmin}); err == nil {
		t.Error("管理员不应能给别人改角色")
	}
	// ④ 管理员不能动超管账号
	if _, err := s.UpdateUser(ctx, *manager, admin.ID, dto.UserUpdateReq{State: model.UserStateDisabled}); err == nil {
		t.Error("管理员不应能停用超管")
	}
	// ⑤ 超管不能改自己的角色（唯一超管自降后再无人可管）
	if _, err := s.UpdateUser(ctx, *admin, admin.ID, dto.UserUpdateReq{Type: model.UserTypeAdmin}); err == nil {
		t.Error("超管不应能改自己的角色")
	}
	// ⑥ 超管把普通用户提成管理员：允许，且落库生效
	got, err := s.UpdateUser(ctx, *admin, plain.ID, dto.UserUpdateReq{Type: model.UserTypeAdmin})
	if err != nil {
		t.Fatalf("超管提升角色应成功: %v", err)
	}
	if got.Type != model.UserTypeAdmin {
		t.Errorf("提升后 type = %d, want %d", got.Type, model.UserTypeAdmin)
	}
	// ⑦ 角色取值必须在校举内
	if _, err := s.UpdateUser(ctx, *admin, plain.ID, dto.UserUpdateReq{Type: 9}); err == nil {
		t.Error("非法角色值应被拒绝")
	}
}

// TestUpdateOwnProfileNotBlocked 前端保存资料时会把当前 state/type 原值一起回传，
// 这些「没有变化」的字段不应触发越权拦截，否则普通用户改昵称都会失败。
func TestUpdateOwnProfileNotBlocked(t *testing.T) {
	t.Parallel()
	s, ctx, admin, _, plain := newUserFixture(t)

	got, err := s.UpdateUser(ctx, *plain, plain.ID, dto.UserUpdateReq{
		NickName: "新昵称",
		Type:     plain.Type, // 原值回传
		State:    plain.State,
	})
	if err != nil {
		t.Fatalf("普通用户改自己昵称应成功: %v", err)
	}
	if got.NickName != "新昵称" {
		t.Errorf("昵称未更新: %q", got.NickName)
	}
	// 管理员改他人资料、type/state 原样回传同样不应被拦
	if _, err := s.UpdateUser(ctx, *admin, plain.ID, dto.UserUpdateReq{
		NickName: "管理员改的", Type: plain.Type, State: plain.State,
	}); err != nil {
		t.Fatalf("管理员改他人昵称应成功: %v", err)
	}
}

// TestListUsersScrubsPrivateFields 用户列表对普通用户只回名字，不回邮箱/手机号/登录信息。
func TestListUsersScrubsPrivateFields(t *testing.T) {
	t.Parallel()
	s, ctx, admin, _, plain := newUserFixture(t)

	res, err := s.ListUsers(ctx, *plain, page.Query{PageSize: 200}, "")
	if err != nil {
		t.Fatalf("ListUsers(普通用户): %v", err)
	}
	if len(res.Items) == 0 {
		t.Fatal("用户列表不应为空")
	}
	for _, u := range res.Items {
		if u.Email != "" || u.MobileNumber != "" || u.LastLoginIP != "" || u.LastLoginTime != nil {
			t.Errorf("普通用户不应看到他人隐私字段: %+v", u)
		}
		if u.Name == "" {
			t.Error("名字仍需返回（备忘清单/作者名回显依赖它）")
		}
	}

	res, err = s.ListUsers(ctx, *admin, page.Query{PageSize: 200}, "")
	if err != nil {
		t.Fatalf("ListUsers(超管): %v", err)
	}
	hasEmail := false
	for _, u := range res.Items {
		if u.Email != "" {
			hasEmail = true
		}
	}
	if !hasEmail {
		t.Error("管理员应能看到邮箱")
	}
}
