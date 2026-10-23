package repository

// 审核 H-15：同名多行属于数据损坏，UserByName 必须拒绝而不是任取一条。

import (
	"context"
	"errors"
	"testing"

	"apidoc/internal/model"
)

func TestUserByNameRejectsDuplicateName(t *testing.T) {
	t.Parallel()
	r := newTestRepo(t)
	ctx := context.Background()

	// 唯一索引落地后同名行本来就插不进去；这里先移除索引，模拟历史脏数据。
	if err := r.DB().Exec("DROP INDEX IF EXISTS uk_user_name_active").Error; err != nil {
		t.Fatalf("drop index: %v", err)
	}
	for _, email := range []string{"a@e.com", "b@e.com"} {
		if err := r.CreateUser(ctx, &model.UserInfo{Name: "alice", Email: email, Pwd: "x"}); err != nil {
			t.Fatalf("create alice(%s): %v", email, err)
		}
	}
	if _, err := r.UserByName(ctx, "alice"); !errors.Is(err, ErrUserNameAmbiguous) {
		t.Fatalf("同名多行应返回 ErrUserNameAmbiguous，got %v", err)
	}
}
