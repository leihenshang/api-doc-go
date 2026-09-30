package repository

import (
	"context"
	"time"

	"apidoc/internal/model"

	"gorm.io/gorm"
)

// AppendChange 在同一事务内落一条变更流水，并回写实体的 last_change_id。
// Postgres 对项目行加锁保证「id 分配与提交」有序（R3）；SQLite 单写天然串行。
func (r *Repository) AppendChange(tx *gorm.DB, projectID uint64, entityType, extUID, op string, actor uint64, device string) (int64, error) {
	db := tx
	if db == nil {
		db = r.db
	}
	// R3：Postgres 下对 project 行 FOR UPDATE，避免并发分配 id 乱序
	if db.Dialector.Name() == "postgres" {
		var proj model.Project
		_ = db.Raw("SELECT * FROM project WHERE id = ? FOR UPDATE", projectID).Scan(&proj).Error
	}
	e := &model.ChangeLog{
		ProjectID:  projectID,
		EntityType: entityType,
		ExtUID:     extUID,
		Op:         op,
		Actor:      actor,
		Device:     device,
	}
	if err := db.Create(e).Error; err != nil {
		return 0, err
	}
	return e.ID, nil
}

// SetLastChange 回写实体的 last_change_id。
func (r *Repository) SetLastChange(tx *gorm.DB, entityType string, id uint64, changeID int64) error {
	db := tx
	if db == nil {
		db = r.db
	}
	var col string
	switch entityType {
	case model.SyncTypeAPI:
		col = "api"
	case model.SyncTypeGroup:
		col = "group"
	case model.SyncTypeDoc:
		col = "doc"
	default:
		return nil
	}
	return db.Table(col).Where("id = ?", id).Update("last_change_id", changeID).Error
}

// LatestChangeID 项目当前游标（change_log 最大 id）。
func (r *Repository) LatestChangeID(ctx context.Context, projectID uint64) (int64, error) {
	var id int64
	err := r.db.WithContext(ctx).Model(&model.ChangeLog{}).
		Where("project_id = ?", projectID).
		Select("COALESCE(MAX(id), 0)").Scan(&id).Error
	return id, err
}

// OldestChangeID 流水窗口下界（gap 判定用）。
func (r *Repository) OldestChangeID(ctx context.Context, projectID uint64) (int64, error) {
	var id int64
	err := r.db.WithContext(ctx).Model(&model.ChangeLog{}).
		Where("project_id = ?", projectID).
		Select("COALESCE(MIN(id), 0)").Scan(&id).Error
	return id, err
}

// ChangesSince 增量取流水（id > since，升序，limit 条）。
func (r *Repository) ChangesSince(ctx context.Context, projectID uint64, since int64, limit int) ([]model.ChangeLog, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var out []model.ChangeLog
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND id > ?", projectID, since).
		Order("id ASC").Limit(limit).
		Find(&out).Error
	return out, err
}

// GetSyncOp 幂等查询。
func (r *Repository) GetSyncOp(ctx context.Context, opID string) (*model.SyncOp, error) {
	var e model.SyncOp
	err := r.db.WithContext(ctx).Where("op_id = ?", opID).First(&e).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// SaveSyncOp 记录 push 结果（幂等）。
func (r *Repository) SaveSyncOp(ctx context.Context, e *model.SyncOp) error {
	return r.db.WithContext(ctx).Create(e).Error
}

// FindByExtUID 按客户端 uid 查条目（含软删，用于 tombstone 判定）。
func (r *Repository) FindApiByExtUID(ctx context.Context, projectID uint64, extUID string) (*model.Api, error) {
	var e model.Api
	err := r.db.WithContext(ctx).Where("project_id = ? AND ext_uid = ?", projectID, extUID).First(&e).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// FindGroupByExtUID 按客户端 uid 查分组。
func (r *Repository) FindGroupByExtUID(ctx context.Context, projectID uint64, extUID string) (*model.Group, error) {
	var e model.Group
	err := r.db.WithContext(ctx).Where("project_id = ? AND ext_uid = ?", projectID, extUID).First(&e).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// FindDocByExtUID 按客户端 uid 查文档。
func (r *Repository) FindDocByExtUID(ctx context.Context, projectID uint64, extUID string) (*model.Doc, error) {
	var e model.Doc
	err := r.db.WithContext(ctx).Where("project_id = ? AND ext_uid = ?", projectID, extUID).First(&e).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// ListGroupsAll 含软删（snapshot/changes 应用用）。
func (r *Repository) ListGroupsAll(ctx context.Context, projectID uint64) ([]model.Group, error) {
	var out []model.Group
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("id").Find(&out).Error
	return out, err
}

// ListApisAll 含软删。
func (r *Repository) ListApisAll(ctx context.Context, projectID uint64) ([]model.Api, error) {
	var out []model.Api
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("id").Find(&out).Error
	return out, err
}

// ListDocsAll 含软删。
func (r *Repository) ListDocsAll(ctx context.Context, projectID uint64) ([]model.Doc, error) {
	var out []model.Doc
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("id").Find(&out).Error
	return out, err
}

// GetProjectAny 含软删/写开关读取。
func (r *Repository) GetProjectAny(ctx context.Context, id uint64) (*model.Project, error) {
	var e model.Project
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// CleanupChangeLog 保留 ≥90 天或 ≥10 万条（取大），返回删除行数。
func (r *Repository) CleanupChangeLog(ctx context.Context) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -90)
	var cnt int64
	if err := r.db.WithContext(ctx).Model(&model.ChangeLog{}).Count(&cnt).Error; err != nil {
		return 0, err
	}
	if cnt <= 100_000 {
		return 0, nil
	}
	// 超出 10 万时，裁掉 90 天前且不在「最新 10 万」内的
	var keepMin int64
	if err := r.db.WithContext(ctx).Model(&model.ChangeLog{}).
		Order("id DESC").Limit(1).Offset(99_999).
		Select("id").Scan(&keepMin).Error; err != nil {
		return 0, err
	}
	res := r.db.WithContext(ctx).
		Where("create_time < ? AND id < ?", cutoff, keepMin).
		Delete(&model.ChangeLog{})
	return res.RowsAffected, res.Error
}

// ---- PAT ----

// CreateUserToken 落一条 PAT。
func (r *Repository) CreateUserToken(ctx context.Context, e *model.UserToken) error {
	return r.db.WithContext(ctx).Create(e).Error
}

// ListUserTokens 我的令牌。
func (r *Repository) ListUserTokens(ctx context.Context, userID uint64) ([]model.UserToken, error) {
	var out []model.UserToken
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id DESC").Find(&out).Error
	return out, err
}

// GetUserTokenByHash 按 hash 取令牌。
func (r *Repository) GetUserTokenByHash(ctx context.Context, hash string) (*model.UserToken, error) {
	var e model.UserToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&e).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// TouchUserToken 更新最后使用时间。
func (r *Repository) TouchUserToken(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.UserToken{}).Where("id = ?", id).
		Update("last_used", time.Now()).Error
}

// RevokeUserToken 吊销。
func (r *Repository) RevokeUserToken(ctx context.Context, userID, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.UserToken{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("revoked", 1).Error
}
