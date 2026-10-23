package service

// 审核 H-15：登录遇到同名历史账号时必须明确报错，不能静默命中其中一条。

import (
	"context"
	"errors"
	"strings"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

func TestLoginRejectsAmbiguousUserName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestService(t)

	first, err := s.Register(ctx, dto.RegisterReq{Name: "alice", Email: "a@e.com", Pwd: "p123456", RePwd: "p123456"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	// 模拟单列唯一索引落地前的历史脏数据。
	if err := s.repo.DB().Exec("DROP INDEX IF EXISTS uk_user_name_active").Error; err != nil {
		t.Fatalf("drop index: %v", err)
	}
	if err := s.repo.CreateUser(ctx, &model.UserInfo{
		Name: "alice", Email: "b@e.com", Pwd: first.Pwd, NickName: "alice2",
	}); err != nil {
		t.Fatalf("create duplicate: %v", err)
	}

	_, err = s.Login(ctx, dto.LoginReq{Name: "alice", Pwd: "p123456"})
	var be *BizError
	if !errors.As(err, &be) || !strings.Contains(be.Msg, "用户名不唯一") {
		t.Fatalf("同名登录应返回明确错误，got %v", err)
	}
}
