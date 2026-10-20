package service

import (
	"context"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// SaveApiSample 新增或更新一条响应示例（按 id 是否为 0 区分）。
// 权限：需要接口所属项目的 update 位。
func (s *Service) SaveApiSample(ctx context.Context, userID uint64, typ int8, req dto.ApiSampleSaveReq) (*model.ApiReturnSample, error) {
	api, err := s.repo.GetApi(ctx, req.ApiID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, api.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	var e *model.ApiReturnSample
	if req.ID > 0 {
		e, err = s.repo.GetApiReturnSample(ctx, req.ID)
		if err != nil {
			return nil, err
		}
		if e.ApiID != api.ID {
			return nil, newBiz(errcode.CodeFailed, "示例不属于该接口")
		}
	} else {
		e = &model.ApiReturnSample{ProjectID: api.ProjectID, ApiID: api.ID, ExtUID: newExtUID()}
	}
	e.Name = req.Name
	e.SampleText = req.SampleText
	e.Status = req.Status
	e.ContentType = req.ContentType
	e.Seq = req.Seq
	if e.Status == 0 {
		e.Status = 200
	}
	if req.ID > 0 {
		if err := s.repo.UpdateApiReturnSample(ctx, e); err != nil {
			return nil, err
		}
	} else {
		if err := s.repo.CreateApiReturnSample(ctx, e); err != nil {
			return nil, err
		}
	}
	s.recordChange(model.SyncTypeExample, e.ExtUID, e.ProjectID, e.ID, model.SyncOpUpsert, userID)
	return e, nil
}

// ListApiSamples 列出某接口的全部响应示例（按 seq 升序）。
func (s *Service) ListApiSamples(ctx context.Context, userID uint64, typ int8, apiID uint64) ([]model.ApiReturnSample, error) {
	a, err := s.repo.GetApi(ctx, apiID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, a.ProjectID, model.PermQuery); err != nil {
		return nil, err
	}
	return s.repo.ListApiReturnSamples(ctx, apiID)
}

// DeleteApiSample 删除一条响应示例。
func (s *Service) DeleteApiSample(ctx context.Context, userID uint64, typ int8, id uint64) error {
	e, err := s.repo.GetApiReturnSample(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, e.ProjectID, model.PermUpdate); err != nil {
		return err
	}
	if err := s.repo.DeleteApiReturnSample(ctx, id); err != nil {
		return err
	}
	s.recordChange(model.SyncTypeExample, e.ExtUID, e.ProjectID, id, model.SyncOpDelete, userID)
	return nil
}
