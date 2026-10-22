package repository

// 同步快照的 DB 侧分页（P2-26）：原来一次性把项目的 6 类实体全读进内存再切片，
// 每翻一页都要重扫全量（O(页数 × N)），大项目上非常慢。这里改为「按类型计数 + 按需分页」，
// 单页代价只与 limit 相关，与项目总量无关。

import (
	"context"

	"apidoc/internal/model"

	"gorm.io/gorm"
)

// countProjectRows 统计某张表里属于该项目的行数（含软删，快照要带上 Deleted 标记）。
//
// 审核 H-8：快照只输出 ext_uid 非空的行（appendItem 会丢掉空 uid），计数必须同口径，
// 否则 total 偏大 → HasMore 恒 true、offset 前进却拿不到新行，客户端分页会原地打转。
func countProjectRows[T any](ctx context.Context, db *gorm.DB, projectID uint64) (int64, error) {
	var n int64
	err := db.WithContext(ctx).Model(new(T)).
		Where("project_id = ? AND ext_uid <> ''", projectID).
		Count(&n).Error
	return n, err
}

// listProjectRowsPage 按 id 稳定排序取一页（只取可参与同步的行，与计数同口径）。
func listProjectRowsPage[T any](ctx context.Context, db *gorm.DB, projectID uint64, offset, limit int) ([]T, error) {
	var out []T
	err := db.WithContext(ctx).
		Where("project_id = ? AND ext_uid <> ''", projectID).
		Order("id").
		Offset(offset).
		Limit(limit).
		Find(&out).Error
	return out, err
}

// extUIDMapByIDs 批量把 id 映射成 ext_uid（一次 IN 查询）。
func extUIDMapByIDs[T any](ctx context.Context, db *gorm.DB, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID     uint64
		ExtUID string
	}
	if err := db.WithContext(ctx).Model(new(T)).
		Select("id, ext_uid").
		Where("id IN ?", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = r.ExtUID
	}
	return out, nil
}

// ---- 分组 ----

// CountGroupsAll 项目内分组总数（含软删）。
func (r *Repository) CountGroupsAll(ctx context.Context, projectID uint64) (int64, error) {
	return countProjectRows[model.Group](ctx, r.db, projectID)
}

// ListGroupsPage 分组分页（按 id）。
func (r *Repository) ListGroupsPage(ctx context.Context, projectID uint64, offset, limit int) ([]model.Group, error) {
	return listProjectRowsPage[model.Group](ctx, r.db, projectID, offset, limit)
}

// GroupExtUIDMap 批量取分组 id → ext_uid。
func (r *Repository) GroupExtUIDMap(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	return extUIDMapByIDs[model.Group](ctx, r.db, ids)
}

// ---- 接口 ----

// CountApisAll 项目内接口总数（含软删）。
func (r *Repository) CountApisAll(ctx context.Context, projectID uint64) (int64, error) {
	return countProjectRows[model.Api](ctx, r.db, projectID)
}

// ListApisPage 接口分页（按 id）。
func (r *Repository) ListApisPage(ctx context.Context, projectID uint64, offset, limit int) ([]model.Api, error) {
	return listProjectRowsPage[model.Api](ctx, r.db, projectID, offset, limit)
}

// ApiExtUIDMap 批量取接口 id → ext_uid（响应示例的 api_uid 用）。
func (r *Repository) ApiExtUIDMap(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	return extUIDMapByIDs[model.Api](ctx, r.db, ids)
}

// ---- 文档 ----

// CountDocsAll 项目内文档总数（含软删）。
func (r *Repository) CountDocsAll(ctx context.Context, projectID uint64) (int64, error) {
	return countProjectRows[model.Doc](ctx, r.db, projectID)
}

// ListDocsPage 文档分页（按 id）。
func (r *Repository) ListDocsPage(ctx context.Context, projectID uint64, offset, limit int) ([]model.Doc, error) {
	return listProjectRowsPage[model.Doc](ctx, r.db, projectID, offset, limit)
}

// ---- 响应示例 ----

// CountApiReturnSamplesAll 项目内响应示例总数（含软删）。
func (r *Repository) CountApiReturnSamplesAll(ctx context.Context, projectID uint64) (int64, error) {
	return countProjectRows[model.ApiReturnSample](ctx, r.db, projectID)
}

// ListApiReturnSamplesPage 响应示例分页（按 id）。
func (r *Repository) ListApiReturnSamplesPage(ctx context.Context, projectID uint64, offset, limit int) ([]model.ApiReturnSample, error) {
	return listProjectRowsPage[model.ApiReturnSample](ctx, r.db, projectID, offset, limit)
}

// ---- 变量环境 / 变量 ----

// CountVarEnvsAll 项目内环境总数（含软删）。
func (r *Repository) CountVarEnvsAll(ctx context.Context, projectID uint64) (int64, error) {
	return countProjectRows[model.VariableEnv](ctx, r.db, projectID)
}

// ListVarEnvsPage 环境分页（按 id）。
func (r *Repository) ListVarEnvsPage(ctx context.Context, projectID uint64, offset, limit int) ([]model.VariableEnv, error) {
	return listProjectRowsPage[model.VariableEnv](ctx, r.db, projectID, offset, limit)
}

// VarEnvExtUIDMap 批量取环境 id → ext_uid（变量 env_uid 用）。
func (r *Repository) VarEnvExtUIDMap(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	return extUIDMapByIDs[model.VariableEnv](ctx, r.db, ids)
}

// CountVariablesAll 项目内变量总数（含软删）。
func (r *Repository) CountVariablesAll(ctx context.Context, projectID uint64) (int64, error) {
	return countProjectRows[model.Variable](ctx, r.db, projectID)
}

// ListVariablesPage 变量分页（按 id）。
func (r *Repository) ListVariablesPage(ctx context.Context, projectID uint64, offset, limit int) ([]model.Variable, error) {
	return listProjectRowsPage[model.Variable](ctx, r.db, projectID, offset, limit)
}

// ---- 字段字典 ----

// CountFieldMappingsAll 项目内字段字典总数（含软删）。
func (r *Repository) CountFieldMappingsAll(ctx context.Context, projectID uint64) (int64, error) {
	return countProjectRows[model.FieldMapping](ctx, r.db, projectID)
}

// ListFieldMappingsPage 字段字典分页（按 id）。
func (r *Repository) ListFieldMappingsPage(ctx context.Context, projectID uint64, offset, limit int) ([]model.FieldMapping, error) {
	return listProjectRowsPage[model.FieldMapping](ctx, r.db, projectID, offset, limit)
}
