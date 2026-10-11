package repository

import (
	"context"

	"apidoc/internal/model"

	"gorm.io/gorm"
)

// ================= 接口响应示例 =================

// CreateApiReturnSample 新建一条响应示例。
func (r *Repository) CreateApiReturnSample(ctx context.Context, v *model.ApiReturnSample) error {
	return gmCreate(ctx, r.db, v)
}

// UpdateApiReturnSample 保存整条响应示例。
func (r *Repository) UpdateApiReturnSample(ctx context.Context, v *model.ApiReturnSample) error {
	return gmSave(ctx, r.db, v)
}

// DeleteApiReturnSample 物理删除一条响应示例。
func (r *Repository) DeleteApiReturnSample(ctx context.Context, id uint64) error {
	return gmHardDelete[model.ApiReturnSample](ctx, r.db, id)
}

// ListApiReturnSamples 取某接口的全部响应示例（按 seq 升序、id 升序）。
func (r *Repository) ListApiReturnSamples(ctx context.Context, apiID uint64) ([]model.ApiReturnSample, error) {
	var out []model.ApiReturnSample
	err := r.db.WithContext(ctx).Where("api_id = ? AND is_deleted = 0", apiID).
		Order("seq ASC, id ASC").Find(&out).Error
	return out, err
}

// GetApiReturnSample 按主键取示例（含软删）。
func (r *Repository) GetApiReturnSample(ctx context.Context, id uint64) (*model.ApiReturnSample, error) {
	return gmGet[model.ApiReturnSample](ctx, r.db, id)
}

// FindApiReturnSampleByExtUID 按客户端 uid 查示例（同步用，含软删）。
func (r *Repository) FindApiReturnSampleByExtUID(ctx context.Context, projectID uint64, extUID string) (*model.ApiReturnSample, error) {
	var e model.ApiReturnSample
	err := r.db.WithContext(ctx).Where("project_id = ? AND ext_uid = ?", projectID, extUID).First(&e).Error
	if err == nil {
		return &e, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

// ListApiReturnSamplesAll 项目下全部示例（含软删，snapshot/changes 用）。
func (r *Repository) ListApiReturnSamplesAll(ctx context.Context, projectID uint64) ([]model.ApiReturnSample, error) {
	var out []model.ApiReturnSample
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("id").Find(&out).Error
	return out, err
}
