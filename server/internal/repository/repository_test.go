package repository

// 仓储层单测：软删除原语（gmSoftDelete/gmRestore/gmPurge）、读路径的 is_deleted
// 过滤、以及 id → ext_uid 反查。这一层此前零测试，而它是「删除到底删了没有」
// 的唯一真值来源 —— service 层测到的是它的行为，出问题时不好定位。

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"apidoc/internal/config"
	"apidoc/internal/database"
	"apidoc/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "repo.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return New(db, &config.Config{})
}

// newGroup 建一个分组并返回其 id。
func newGroup(t *testing.T, r *Repository, projectID uint64, title string) *model.Group {
	t.Helper()
	g := &model.Group{ProjectID: projectID, Title: title, Type: model.GroupTypeAPI, ExtUID: "uid-" + title}
	if err := r.CreateGroup(context.Background(), g); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	return g
}

func TestGroupSoftDeleteLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newTestRepo(t)
	g := newGroup(t, r, 1, "用户")

	// 活数据：普通读能拿到。
	if _, err := r.GetGroup(ctx, g.ID); err != nil {
		t.Fatalf("GetGroup: %v", err)
	}

	// 软删除后：普通读必须拒绝，Any 读仍可见（回收站/还原路径要用）。
	if err := r.DeleteGroup(ctx, g.ID); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if _, err := r.GetGroup(ctx, g.ID); err == nil {
		t.Fatal("软删除后 GetGroup 应返回不存在")
	}
	got, err := r.GetGroupAny(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetGroupAny: %v", err)
	}
	if got.IsDeleted != model.IsDeletedYes {
		t.Fatalf("is_deleted 应为 1，got %d", got.IsDeleted)
	}

	// 列表默认排除已删。
	list, err := r.ListGroups(ctx, 1, model.GroupTypeAPI)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("列表不应包含已删分组，got %+v", list)
	}

	// 还原后重新可见。
	if n, err := r.RestoreGroups(ctx, []uint64{g.ID}); err != nil || n != 1 {
		t.Fatalf("RestoreGroups: n=%d err=%v", n, err)
	}
	if _, err := r.GetGroup(ctx, g.ID); err != nil {
		t.Fatalf("还原后 GetGroup 应可读: %v", err)
	}

	// 彻底删除：行真的没了，连 Any 也查不到。
	if n, err := r.PurgeGroups(ctx, []uint64{g.ID}); err != nil || n != 1 {
		t.Fatalf("PurgeGroups: n=%d err=%v", n, err)
	}
	if _, err := r.GetGroupAny(ctx, g.ID); err == nil {
		t.Fatal("彻底删除后不应还能读到（说明只是又软删了一次）")
	}
}

func TestListGroupsAllowsAnyType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newTestRepo(t)
	apiGroup := newGroup(t, r, 1, "接口组")
	docGroup := &model.Group{ProjectID: 1, Title: "文档组", Type: model.GroupTypeDoc, ExtUID: "uid-doc"}
	if err := r.CreateGroup(ctx, docGroup); err != nil {
		t.Fatalf("CreateGroup doc: %v", err)
	}

	onlyAPI, err := r.ListGroups(ctx, 1, model.GroupTypeAPI)
	if err != nil {
		t.Fatalf("ListGroups api: %v", err)
	}
	if len(onlyAPI) != 1 || onlyAPI[0].ID != apiGroup.ID {
		t.Fatalf("按类型过滤应只返回接口组，got %+v", onlyAPI)
	}

	// typ < 0 = 不按类型过滤：接口与文档分组可共用（回收站还原祖先链要用）。
	all, err := r.ListGroups(ctx, 1, -1)
	if err != nil {
		t.Fatalf("ListGroups all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("不按类型过滤应返回 2 个分组，got %+v", all)
	}
}

func TestExtUIDByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newTestRepo(t)
	g := newGroup(t, r, 1, "用户")

	if got := r.GroupExtUID(ctx, g.ID); got != g.ExtUID {
		t.Fatalf("GroupExtUID = %q, want %q", got, g.ExtUID)
	}
	// id 为 0 / 不存在 → 空串，不报错（拼同步载荷时缺引用是合法状态）。
	if got := r.GroupExtUID(ctx, 0); got != "" {
		t.Fatalf("id=0 应返回空串，got %q", got)
	}
	if got := r.GroupExtUID(ctx, 9999); got != "" {
		t.Fatalf("不存在的 id 应返回空串，got %q", got)
	}

	// 软删后仍要能反查到（子条目的 parent_uid 必须指向真实父 uid）。
	if err := r.DeleteGroup(ctx, g.ID); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if got := r.GroupExtUID(ctx, g.ID); got != g.ExtUID {
		t.Fatalf("软删后仍应反查到 ext_uid，got %q", got)
	}
}

func TestSoftDeleteByIDsEmptyInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newTestRepo(t)
	// 空 id 列表不应拼出 "WHERE id IN ()" 这种非法 SQL。
	if n, err := gmSoftDeleteByIDs[model.Group](ctx, r.db, nil); err != nil || n != 0 {
		t.Fatalf("空列表应 no-op：n=%d err=%v", n, err)
	}
}

// TestAddMemberIdempotent 同一 (project_id, user_id) 只允许一行活数据（P1-9）。
// 并发添加同一成员时不得出现重复行，重复调用应是「更新」而不是「再插一行」。
func TestAddMemberIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newTestRepo(t)

	// 串行重复调用：第二次应更新权限而不是插新行。
	if err := r.AddMember(ctx, &model.UserProject{UserID: 1, ProjectID: 10, Permission: model.PermReadonly}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if err := r.AddMember(ctx, &model.UserProject{UserID: 1, ProjectID: 10, Permission: model.PermUpdate, IsLeader: 1}); err != nil {
		t.Fatalf("AddMember(again): %v", err)
	}
	var count int64
	if err := r.db.Model(&model.UserProject{}).
		Where("user_id = ? AND project_id = ? AND is_deleted = 0", 1, 10).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("重复添加不应产生第二行，got %d 行", count)
	}
	m, err := r.Member(ctx, 1, 10)
	if err != nil {
		t.Fatalf("Member: %v", err)
	}
	if m.Permission != model.PermUpdate || m.IsLeader != 1 {
		t.Fatalf("第二次调用应更新权限/负责人，got %+v", m)
	}

	// 并发添加同一成员：唯一索引必须挡住重复行。
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- r.AddMember(ctx, &model.UserProject{UserID: 2, ProjectID: 10, Permission: model.PermReadonly})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("并发 AddMember 出错: %v", err)
		}
	}
	if err := r.db.Model(&model.UserProject{}).
		Where("user_id = ? AND project_id = ? AND is_deleted = 0", 2, 10).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("并发添加同一成员只应留 1 行，got %d 行", count)
	}
}

// TestAddMemberAfterSoftDelete 之前被软删的成员重新添加时，不应堆积死行。
func TestAddMemberAfterSoftDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newTestRepo(t)
	if err := r.AddMember(ctx, &model.UserProject{UserID: 3, ProjectID: 20, Permission: model.PermReadonly}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	// 软删该成员关系
	if err := r.db.Model(&model.UserProject{}).
		Where("user_id = ? AND project_id = ?", 3, 20).
		Update("is_deleted", model.IsDeletedYes).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	// 重新添加：应恢复为活行
	if err := r.AddMember(ctx, &model.UserProject{UserID: 3, ProjectID: 20, Permission: model.PermUpdate}); err != nil {
		t.Fatalf("AddMember(重新添加): %v", err)
	}
	var total, live int64
	if err := r.db.Unscoped().Model(&model.UserProject{}).
		Where("user_id = ? AND project_id = ?", 3, 20).Count(&total).Error; err != nil {
		t.Fatalf("count total: %v", err)
	}
	if err := r.db.Model(&model.UserProject{}).
		Where("user_id = ? AND project_id = ? AND is_deleted = 0", 3, 20).Count(&live).Error; err != nil {
		t.Fatalf("count live: %v", err)
	}
	if live != 1 || total != 1 {
		t.Fatalf("重新添加后应只有 1 行且为活行，got live=%d total=%d", live, total)
	}
	m, err := r.Member(ctx, 3, 20)
	if err != nil || m.Permission != model.PermUpdate {
		t.Fatalf("重新添加应按新权限生效，got %+v (%v)", m, err)
	}
}
