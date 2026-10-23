package service

// 审核 H-21：EnsureAdmin 不得重置已有超管口令；seed -force 走显式 ResetAdminPassword。

import (
	"context"
	"testing"

	"apidoc/internal/dto"
)

func TestEnsureAdminDoesNotResetPassword(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestService(t)

	if _, err := s.InitAdmin(ctx, "admin", "oldpass"); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	created, err := s.EnsureAdmin(ctx, "admin", "newpass")
	if err != nil {
		t.Fatalf("EnsureAdmin: %v", err)
	}
	if created {
		t.Fatal("已存在超管时 EnsureAdmin 应返回 created=false")
	}
	if _, err := s.Login(ctx, dto.LoginReq{Name: "admin", Pwd: "oldpass"}); err != nil {
		t.Fatalf("EnsureAdmin 不应重置口令，旧口令应仍可登录: %v", err)
	}
	if _, err := s.Login(ctx, dto.LoginReq{Name: "admin", Pwd: "newpass"}); err == nil {
		t.Fatal("EnsureAdmin 不应把口令改成新值")
	}

	if _, err := s.ResetAdminPassword(ctx, "admin", "newpass"); err != nil {
		t.Fatalf("ResetAdminPassword: %v", err)
	}
	if _, err := s.Login(ctx, dto.LoginReq{Name: "admin", Pwd: "newpass"}); err != nil {
		t.Fatalf("显式重置后新口令应可登录: %v", err)
	}
}
