package service

// U6：集合级公共参数（项目级单例）。服务端权威，客户端只读镜像。
// 存 JSON 文本、服务端不解释结构 —— 解释责任在前端编辑器与客户端的 MergeDefaults。

import (
	"context"
	"encoding/json"
	"strings"

	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// GetProjectDefaults 读项目公共参数（未设置返回空串）。
func (s *Service) GetProjectDefaults(ctx context.Context, userID uint64, typ int8, projectID uint64) (string, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermQuery); err != nil {
		return "", err
	}
	p, err := s.repo.ProjectByID(ctx, projectID)
	if err != nil {
		return "", err
	}
	return p.Defaults, nil
}

// SetProjectDefaults 写项目公共参数（传空串 = 清空）。
func (s *Service) SetProjectDefaults(ctx context.Context, userID uint64, typ int8, projectID uint64, raw string) error {
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermUpdate); err != nil {
		return err
	}
	raw = strings.TrimSpace(raw)
	// 空串允许（清空）；非空必须是合法 JSON，避免把半边文本存进库、同步时客户端解析失败。
	if raw != "" && !json.Valid([]byte(raw)) {
		return newBiz(errcode.CodeParamError, "公共参数必须是合法 JSON")
	}
	p, err := s.repo.ProjectByID(ctx, projectID)
	if err != nil {
		return err
	}
	p.Defaults = raw
	if err := s.repo.UpdateProject(ctx, p); err != nil {
		return err
	}
	s.recordOp(ctx, projectID, userID, projectID, model.LogObjectProject, "defaults", "更新集合公共参数")
	return nil
}
