package service

import (
	"context"
	"time"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/jwt"
	"apidoc/internal/pkg/page"
	"apidoc/internal/pkg/password"
)

// LoginResult 是登录结果。
type LoginResult struct {
	Token string          `json:"token"`
	User  *model.UserInfo `json:"user"`
}

// Login 校验凭据并签发 JWT。
func (s *Service) Login(ctx context.Context, req dto.LoginReq) (*LoginResult, error) {
	u, err := s.repo.UserByName(ctx, req.Name)
	if err != nil {
		return nil, newBiz(errcode.CodeFailed, "用户名或密码错误")
	}
	ok, err := password.Verify(req.Pwd, u.Pwd)
	if err != nil || !ok {
		return nil, newBiz(errcode.CodeFailed, "用户名或密码错误")
	}
	if u.State == model.UserStateDisabled {
		return nil, newBiz(errcode.CodeForbidden, "账号已禁用")
	}
	token, err := jwt.Generate(u.ID, u.Type, "apidoc", s.cfg.JWT.Secret, hours(s.cfg.JWT.ExpireHours))
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, User: u}, nil
}

// Register 新建普通用户。
func (s *Service) Register(ctx context.Context, req dto.RegisterReq) (*model.UserInfo, error) {
	if req.Pwd != req.RePwd {
		return nil, newBiz(errcode.CodeParamError, "两次输入的密码不一致")
	}
	taken, err := s.repo.UserTaken(ctx, req.Name, req.Email, req.NickName, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, newBiz(errcode.CodeFailed, "用户名或邮箱重复")
	}
	hash, err := password.Generate(req.Pwd)
	if err != nil {
		return nil, err
	}
	nick := req.NickName
	if nick == "" {
		nick = "新用户-" + req.Name
	}
	u := &model.UserInfo{
		Name: req.Name, Pwd: hash, Email: req.Email, NickName: nick,
		Type: model.UserTypeNormal, State: model.UserStateNormal,
	}
	if err := s.repo.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// GetUser 按主键取用户。
func (s *Service) GetUser(ctx context.Context, id uint64) (*model.UserInfo, error) {
	return s.repo.UserByID(ctx, id)
}

// ListUsers 分页查询用户。
func (s *Service) ListUsers(ctx context.Context, q page.Query, keyword string) (page.Result[model.UserInfo], error) {
	q.Normalize()
	items, total, err := s.repo.ListUsers(ctx, q, keyword)
	if err != nil {
		return page.Result[model.UserInfo]{}, err
	}
	return page.NewResult(items, total, q), nil
}

// CheckRepeat 关键字是否已被占用。
func (s *Service) CheckRepeat(ctx context.Context, keyword string) (bool, error) {
	return s.repo.UserTaken(ctx, keyword, keyword, keyword, 0)
}

// UpdateUser 更新资料；仅管理员可改 type/state。
func (s *Service) UpdateUser(ctx context.Context, operator model.UserInfo, targetID uint64, req dto.UserUpdateReq) (*model.UserInfo, error) {
	u, err := s.repo.UserByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if operator.ID != targetID && !isAdmin(operator.Type) {
		return nil, newBiz(errcode.CodeForbidden, "无权修改他人资料")
	}
	if req.NickName != "" {
		u.NickName = req.NickName
	}
	if req.Email != "" {
		u.Email = req.Email
	}
	if req.UserFace != "" {
		u.UserFace = req.UserFace
	}
	if isAdmin(operator.Type) {
		if req.State > 0 {
			u.State = req.State
		}
		if req.Type > 0 {
			u.Type = req.Type
		}
	}
	if err := s.repo.UpdateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// UpdatePwd 修改本人密码。
func (s *Service) UpdatePwd(ctx context.Context, userID uint64, req dto.UpdatePwdReq) error {
	if req.Pwd != req.RePwd {
		return newBiz(errcode.CodeParamError, "两次输入的密码不一致")
	}
	u, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	ok, err := password.Verify(req.OldPwd, u.Pwd)
	if err != nil || !ok {
		return newBiz(errcode.CodeFailed, "原密码错误")
	}
	hash, err := password.Generate(req.Pwd)
	if err != nil {
		return err
	}
	u.Pwd = hash
	return s.repo.UpdateUser(ctx, u)
}

// InitAdmin 初始化/重置超级管理员（幂等），供 seed 命令调用。
func (s *Service) InitAdmin(ctx context.Context, name, plainPwd string) (*model.UserInfo, error) {
	hash, err := password.Generate(plainPwd)
	if err != nil {
		return nil, err
	}
	u, err := s.repo.UserByName(ctx, name)
	if err == nil {
		u.Pwd = hash
		u.Type = model.UserTypeSuperuser
		u.State = model.UserStateNormal
		if err := s.repo.UpdateUser(ctx, u); err != nil {
			return nil, err
		}
		return u, nil
	}
	u = &model.UserInfo{
		Name: name, Pwd: hash, NickName: name,
		Type: model.UserTypeSuperuser, State: model.UserStateNormal,
	}
	if err := s.repo.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// EnsureAdmin 保证默认超管存在：仅当登录名不存在时创建（返回 created=true），
// 不重置已有账号密码；需要强制重置请改用 InitAdmin。
func (s *Service) EnsureAdmin(ctx context.Context, name, plainPwd string) (bool, error) {
	if _, err := s.repo.UserByName(ctx, name); err == nil {
		return false, nil
	}
	_, err := s.InitAdmin(ctx, name, plainPwd)
	return err == nil, err
}

// hours 将小时数转为 time.Duration。
func hours(n int) time.Duration { return time.Duration(n) * time.Hour }
