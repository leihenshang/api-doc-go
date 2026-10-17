package service

import (
	"context"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

// TestAppendChangeTxRollsBackOnSetLastChangeError 流水与修订号必须同事务：
// SetLastChange 失败时，AppendChange 写下的那行 change_log 必须一起回滚，
// 否则会留下「有流水、实体 revision 没前进」的不一致（B-12 / B-15）。
// 这里用未知实体类型触发 SetLastChange 的显式报错来制造失败点。
func TestAppendChangeTxRollsBackOnSetLastChangeError(t *testing.T) {
	t.Parallel()
	s := newTestService(t)
	ctx := context.Background()

	if _, err := s.appendChangeTx(ctx, nil, "unknown_entity", "uid-rollback", 1, 1, model.SyncOpUpsert, 1, "test"); err == nil {
		t.Fatal("未知实体类型应让 SetLastChange 报错")
	}

	var n int64
	if err := s.repo.DB().Model(&model.ChangeLog{}).
		Where("ext_uid = ?", "uid-rollback").Count(&n).Error; err != nil {
		t.Fatalf("统计 change_log: %v", err)
	}
	if n != 0 {
		t.Errorf("事务应整体回滚，change_log 仍留下 %d 条", n)
	}
}

// TestAppendChangeTxWritesBoth 成功路径：流水与 last_change_id 都要落库，且指向同一条流水。
func TestAppendChangeTxWritesBoth(t *testing.T) {
	t.Parallel()
	s, ctx, admin, _, _ := newUserFixture(t)

	proj, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "change-log-proj"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	g, err := s.CreateGroup(ctx, admin.ID, admin.Type, dto.GroupCreateReq{
		ProjectID: proj.ID, Title: "目录", Type: model.GroupTypeAPI,
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	// CreateGroup 内部走 recordChange，分组 revision 必须已推进
	got, err := s.repo.GetGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if got.LastChangeID == 0 {
		t.Fatal("分组创建后 last_change_id 应已推进")
	}

	var n int64
	if err := s.repo.DB().Model(&model.ChangeLog{}).
		Where("ext_uid = ? AND op = ?", g.ExtUID, model.SyncOpUpsert).Count(&n).Error; err != nil {
		t.Fatalf("统计 change_log: %v", err)
	}
	if n != 1 {
		t.Errorf("应按 ext_uid 落 1 条 upsert 流水，实际 %d 条", n)
	}

	// last_change_id 必须正好指向那条流水，否则客户端按 revision 比对会错位
	var cid int64
	if err := s.repo.DB().Model(&model.ChangeLog{}).
		Where("ext_uid = ?", g.ExtUID).Select("id").Scan(&cid).Error; err != nil {
		t.Fatalf("取流水 id: %v", err)
	}
	if got.LastChangeID != cid {
		t.Errorf("last_change_id = %d, 应为流水 id %d", got.LastChangeID, cid)
	}
}
