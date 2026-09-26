// 分享链接：为项目生成**免登录只读**访问入口（令牌随机、可设有效期、可随时失效）。
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// ShareProject 分享页展示的项目信息（不含权限等内部字段）。
type ShareProject struct {
	ID          uint64 `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// ShareView 分享只读数据：项目 + 分组 + 接口 + 文档。
type ShareView struct {
	Project    ShareProject  `json:"project"`
	Groups     []model.Group `json:"groups"`
	Apis       []model.Api   `json:"apis"`
	Docs       []model.Doc   `json:"docs"`
	Remark     string        `json:"remark"`
	ExpireTime *time.Time    `json:"expire_time"`
}

// CreateShareLink 生成分享令牌；expire_days > 0 时按天数设置有效期（0 = 永久）。
func (s *Service) CreateShareLink(ctx context.Context, userID uint64, typ int8, req dto.ShareLinkReq) (*model.ShareLink, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	token, err := shareToken()
	if err != nil {
		return nil, err
	}
	link := &model.ShareLink{
		ProjectID: req.ProjectID,
		UserID:    userID,
		Token:     token,
		Remark:    strings.TrimSpace(req.Remark),
	}
	if req.ExpireDays > 0 {
		expire := time.Now().AddDate(0, 0, req.ExpireDays)
		link.ExpireTime = &expire
	}
	if err := s.repo.CreateShareLink(ctx, link); err != nil {
		return nil, err
	}
	s.recordOp(ctx, link.ProjectID, userID, link.ID, model.LogObjectProject, "share", "分享链接:"+link.Remark)
	return link, nil
}

// ListShareLinks 项目下的分享链接。
func (s *Service) ListShareLinks(ctx context.Context, projectID uint64) ([]model.ShareLink, error) {
	if projectID == 0 {
		return nil, newBiz(errcode.CodeParamError, "缺少 project_id")
	}
	return s.repo.ListShareLinks(ctx, projectID)
}

// DeleteShareLink 失效一个分享链接（软删除）。
func (s *Service) DeleteShareLink(ctx context.Context, userID uint64, typ int8, id uint64) error {
	link, err := s.repo.ShareLinkByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, link.ProjectID, model.PermUpdate); err != nil {
		return err
	}
	if err := s.repo.DeleteShareLink(ctx, id); err != nil {
		return err
	}
	s.recordOp(ctx, link.ProjectID, userID, link.ID, model.LogObjectProject, "share-delete", "分享链接:"+link.Remark)
	return nil
}

// SharePayload 公开只读数据（免登录）：校验令牌与有效期后返回项目/分组/接口/文档。
func (s *Service) SharePayload(ctx context.Context, token string) (*ShareView, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, newBiz(errcode.CodeParamError, "缺少分享令牌")
	}
	link, err := s.repo.ShareLinkByToken(ctx, token)
	if err != nil {
		return nil, newBiz(errcode.CodeFailed, "分享链接不存在或已失效")
	}
	if link.ExpireTime != nil && time.Now().After(*link.ExpireTime) {
		return nil, newBiz(errcode.CodeFailed, "分享链接已过期")
	}
	p, err := s.repo.ProjectByID(ctx, link.ProjectID)
	if err != nil {
		return nil, newBiz(errcode.CodeFailed, "分享的项目不存在或已删除")
	}
	groups, err := s.repo.ListGroups(ctx, link.ProjectID, -1)
	if err != nil {
		return nil, err
	}
	apis, err := s.repo.ListApisByProject(ctx, link.ProjectID)
	if err != nil {
		return nil, err
	}
	docs, err := s.repo.DocsByProject(ctx, link.ProjectID)
	if err != nil {
		return nil, err
	}
	return &ShareView{
		Project: ShareProject{
			ID: p.ID, Title: p.Title, Description: p.Description, Version: p.Version,
		},
		Groups:     groups,
		Apis:       apis,
		Docs:       docs,
		Remark:     link.Remark,
		ExpireTime: link.ExpireTime,
	}, nil
}

// shareToken 生成 16 字节随机令牌（32 位十六进制）。
func shareToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
