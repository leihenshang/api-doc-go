package service

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"context"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/page"
)

// ---- Bug ----

// CreateBug 新建缺陷。
func (s *Service) CreateBug(ctx context.Context, userID uint64, typ int8, req dto.BugCreateReq) (*model.Bug, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermCreate); err != nil {
		return nil, err
	}
	b := &model.Bug{ProjectID: req.ProjectID, Title: req.Title, Content: req.Content, Level: req.Level, Comment: req.Comment, Status: model.BugStatusPending}
	if err := s.repo.CreateBug(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// UpdateBug 更新缺陷。
func (s *Service) UpdateBug(ctx context.Context, userID uint64, typ int8, req dto.BugUpdateReq) (*model.Bug, error) {
	b, err := s.repo.GetBug(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, b.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	if req.Title != "" {
		b.Title = req.Title
	}
	if req.Content != "" {
		b.Content = req.Content
	}
	if req.Level > 0 {
		b.Level = req.Level
	}
	if req.Status > 0 {
		b.Status = req.Status
	}
	b.Comment = req.Comment
	if err := s.repo.UpdateBug(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// AssignBug 指派缺陷处理人并记录流转。
func (s *Service) AssignBug(ctx context.Context, userID uint64, typ int8, req dto.BugAssignReq) error {
	b, err := s.repo.GetBug(ctx, req.BugID)
	if err != nil {
		return err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, b.ProjectID, model.PermUpdate); err != nil {
		return err
	}
	b.ToUserID = req.ToUserID
	if err := s.repo.UpdateBug(ctx, b); err != nil {
		return err
	}
	if err := s.repo.CreateBugAssign(ctx, &model.BugAssign{
		BugID: b.ID, FromUserID: userID, ToUserID: req.ToUserID, Comment: req.Comment, Status: model.BugStatusPending,
	}); err != nil {
		return err
	}
	// 指派后给处理人发站内通知；通知失败一并返回，交由调用方感知
	return s.Notify(ctx, req.ToUserID, "Bug 待处理", fmt.Sprintf("你被指派处理缺陷：%s", b.Title))
}

// GetBug 缺陷详情。
func (s *Service) GetBug(ctx context.Context, id uint64) (*model.Bug, error) {
	return s.repo.GetBug(ctx, id)
}

// ListBugs 分页查询缺陷。
func (s *Service) ListBugs(ctx context.Context, projectID uint64, status int8, q page.Query) (page.Result[model.Bug], error) {
	q.Normalize()
	items, total, err := s.repo.ListBugs(ctx, projectID, status, q)
	if err != nil {
		return page.Result[model.Bug]{}, err
	}
	return page.NewResult(items, total, q), nil
}

// ListBugAssign 指派记录。
func (s *Service) ListBugAssign(ctx context.Context, bugID uint64) ([]model.BugAssign, error) {
	return s.repo.ListBugAssign(ctx, bugID)
}

// DeleteBug 删除缺陷。
func (s *Service) DeleteBug(ctx context.Context, userID uint64, typ int8, id uint64) error {
	b, err := s.repo.GetBug(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, b.ProjectID, model.PermDelete); err != nil {
		return err
	}
	return s.repo.DeleteBug(ctx, id)
}

// ---- 公共属性（管理员） ----

// ListProperties 列出公共属性。
func (s *Service) ListProperties(ctx context.Context) ([]model.Property, error) {
	return s.repo.ListProperties(ctx)
}

// SaveProperty 新建或更新属性（仅管理员）。
func (s *Service) SaveProperty(ctx context.Context, typ int8, req dto.PropertyReq) (*model.Property, error) {
	if !isAdmin(typ) {
		return nil, newBiz(errcode.CodeForbidden, "仅管理员可维护公共属性")
	}
	p := &model.Property{ID: req.ID, Tag: req.Tag, TagName: req.TagName, Description: req.Description}
	if req.ID == 0 {
		if err := s.repo.CreateProperty(ctx, p); err != nil {
			return nil, err
		}
		return p, nil
	}
	if err := s.repo.UpdateProperty(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// DeleteProperty 删除属性（仅管理员）。
func (s *Service) DeleteProperty(ctx context.Context, typ int8, id uint64) error {
	if !isAdmin(typ) {
		return newBiz(errcode.CodeForbidden, "仅管理员可维护公共属性")
	}
	return s.repo.DeleteProperty(ctx, id)
}

// ---- 消息 / 验证码 ----

// SendMessage 生成并保存验证码（邮件/短信通道未接入时仅落库，供开发联调）。
func (s *Service) SendMessage(ctx context.Context, req dto.MessageSendReq) error {
	code, err := randomCode(6)
	if err != nil {
		return err
	}
	m := &model.Message{
		ReceiveSource: req.ReceiveSource, SendType: req.SendType, Code: code, Title: "验证码",
		Content:    fmt.Sprintf("你的验证码是 %s，10 分钟内有效。", code),
		ExpireTime: time.Now().Add(10 * time.Minute),
	}
	return s.repo.CreateMessage(ctx, m)
}

// randomCode 生成 n 位数字验证码。
func randomCode(n int) (string, error) {
	var sb [6]byte
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		sb[i] = byte('0' + v.Int64())
	}
	return string(sb[:n]), nil
}

// ---- 站内通知 ----

// Notify 创建站内通知。
func (s *Service) Notify(ctx context.Context, receiver uint64, title, content string) error {
	return s.repo.CreateNotification(ctx, &model.Notification{Receiver: receiver, Title: title, Content: content, SendType: 1})
}

// ListNotifications 分页列出收件人通知。
func (s *Service) ListNotifications(ctx context.Context, receiver uint64, q page.Query) (page.Result[model.Notification], error) {
	q.Normalize()
	items, total, err := s.repo.ListNotifications(ctx, receiver, q)
	if err != nil {
		return page.Result[model.Notification]{}, err
	}
	return page.NewResult(items, total, q), nil
}

// MarkNotificationRead 标记通知已读。
func (s *Service) MarkNotificationRead(ctx context.Context, receiver, id uint64) error {
	return s.repo.MarkNotificationRead(ctx, id)
}

// ---- 操作日志 ----

// ListLogs 分页查询操作日志。
func (s *Service) ListLogs(ctx context.Context, projectID, objectID uint64, typ int8, q page.Query) (page.Result[model.OperationLog], error) {
	q.Normalize()
	items, total, err := s.repo.ListLogs(ctx, projectID, objectID, typ, q)
	if err != nil {
		return page.Result[model.OperationLog]{}, err
	}
	return page.NewResult(items, total, q), nil
}
