package service

// 审核 H-16：AssignBug 的「改指派人 + 写流转记录」必须同事务；通知失败不得回滚已成功的指派。

import (
	"testing"

	"apidoc/internal/dto"
)

// TestAssignBugNotifyBestEffort 通知插入失败时，指派仍应成功并落库。
func TestAssignBugNotifyBestEffort(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx
	bug, err := f.s.CreateBug(ctx, f.admin.ID, f.admin.Type, dto.BugCreateReq{
		ProjectID: f.proj.ID, Title: "登录失败", Content: "偶现 500",
	})
	if err != nil {
		t.Fatalf("CreateBug: %v", err)
	}
	// 让 notification 插入必然失败，模拟通知通道异常。
	if err := f.s.repo.DB().Exec(
		`CREATE TRIGGER fail_notification BEFORE INSERT ON notification BEGIN SELECT RAISE(ABORT, 'notify failed'); END;`,
	).Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	if err := f.s.AssignBug(ctx, f.admin.ID, f.admin.Type, dto.BugAssignReq{
		BugID: bug.ID, ToUserID: f.admin.ID, Comment: "请处理",
	}); err != nil {
		t.Fatalf("通知失败不应让指派失败: %v", err)
	}
	got, err := f.s.repo.GetBug(ctx, bug.ID)
	if err != nil || got.ToUserID != f.admin.ID {
		t.Fatalf("指派人未落库: %+v (err=%v)", got, err)
	}
	assigns, err := f.s.repo.ListBugAssign(ctx, bug.ID)
	if err != nil || len(assigns) != 1 || assigns[0].ToUserID != f.admin.ID {
		t.Fatalf("流转记录未落库: %+v (err=%v)", assigns, err)
	}
}

// TestAssignBugRollsBackOnAssignFailure 流转记录写入失败时，指派人修改必须回滚。
func TestAssignBugRollsBackOnAssignFailure(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx
	bug, err := f.s.CreateBug(ctx, f.admin.ID, f.admin.Type, dto.BugCreateReq{
		ProjectID: f.proj.ID, Title: "支付超时", Content: "回调丢失",
	})
	if err != nil {
		t.Fatalf("CreateBug: %v", err)
	}
	if err := f.s.repo.DB().Exec(
		`CREATE TRIGGER fail_bug_assign BEFORE INSERT ON bug_assign BEGIN SELECT RAISE(ABORT, 'assign failed'); END;`,
	).Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	if err := f.s.AssignBug(ctx, f.admin.ID, f.admin.Type, dto.BugAssignReq{
		BugID: bug.ID, ToUserID: f.admin.ID, Comment: "请处理",
	}); err == nil {
		t.Fatal("流转记录写入失败应返回错误")
	}
	got, err := f.s.repo.GetBug(ctx, bug.ID)
	if err != nil {
		t.Fatalf("GetBug: %v", err)
	}
	if got.ToUserID != 0 {
		t.Fatalf("事务应回滚指派人修改，got to_user_id=%d", got.ToUserID)
	}
	assigns, err := f.s.repo.ListBugAssign(ctx, bug.ID)
	if err != nil || len(assigns) != 0 {
		t.Fatalf("事务回滚后不应有流转记录: %+v (err=%v)", assigns, err)
	}
}
