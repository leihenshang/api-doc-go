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
	token, err := jwt.Generate(u.ID, u.Type, jwt.Issuer, s.cfg.JWT.Secret, hours(s.cfg.JWT.ExpireHours))
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

// LoadPrincipal 按用户 ID 回源刷新当前权限主体（审核 H-1）。
// JWT 里的 Type 是签发时刻的快照：管理员禁用/降权后存量令牌仍会放行，因此鉴权中间件
// 每请求用它校一次账号状态并取最新类型（数据库为唯一真源）。账号不存在/已禁用 → 拒绝。
func (s *Service) LoadPrincipal(ctx context.Context, userID uint64) (int8, error) {
	u, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return 0, newBiz(errcode.CodeUnauthorized, "登录状态已失效")
	}
	if u.State == model.UserStateDisabled {
		return 0, newBiz(errcode.CodeForbidden, "账号已禁用")
	}
	return u.Type, nil
}

// ListUsers 分页查询用户。
// ListUsers 用户列表。所有登录用户可读——备忘清单指派、成员选择、作者名回显都需要
// 把 user_id 映射成名字；但邮箱 / 手机号 / 最近登录信息属个人信息，只对管理员可见，
// 否则任意普通账号都能拉走整张用户表。
func (s *Service) ListUsers(ctx context.Context, operator model.UserInfo, q page.Query, keyword string) (page.Result[model.UserInfo], error) {
	q.Normalize()
	items, total, err := s.repo.ListUsers(ctx, q, keyword)
	if err != nil {
		return page.Result[model.UserInfo]{}, err
	}
	if !isAdmin(operator.Type) {
		for i := range items {
			items[i].Email = ""
			items[i].MobileNumber = ""
			items[i].LastLoginIP = ""
			items[i].LastLoginTime = nil
		}
	}
	return page.NewResult(items, total, q), nil
}

// CheckRepeat 关键字是否已被占用。
func (s *Service) CheckRepeat(ctx context.Context, keyword string) (bool, error) {
	return s.repo.UserTaken(ctx, keyword, keyword, keyword, 0)
}

// UpdateUser 更新资料；仅管理员可改 type/state，且角色变更另受 ensureAccountChangeAllowed 约束。
func (s *Service) UpdateUser(ctx context.Context, operator model.UserInfo, targetID uint64, req dto.UserUpdateReq) (*model.UserInfo, error) {
	u, err := s.repo.UserByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if operator.ID != targetID && !isAdmin(operator.Type) {
		return nil, newBiz(errcode.CodeForbidden, "无权修改他人资料")
	}
	// 越权判定必须放在赋值之前：一旦先写进 u 再校验，失败路径也会把改动带出去。
	if err := ensureAccountChangeAllowed(operator, u, req); err != nil {
		return nil, err
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
	// 界面语言：本人即可改（与昵称/邮箱同级），且只接受白名单取值。
	if req.Lang != "" {
		if !model.IsSupportedLang(req.Lang) {
			return nil, newBiz(errcode.CodeParamError, "不支持的界面语言：%s", req.Lang)
		}
		u.Lang = req.Lang
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

// ensureAccountChangeAllowed 校验角色 / 状态变更的越权边界（W-05）。
// 只比较「是否真的变」：前端保存资料时会把当前 state/type 原值一起回传，
// 若按 `> 0` 判断，普通用户改自己昵称也会被误拦。
//
// 三条边界：
//  1. 非管理员不得改任何人的角色或状态；
//  2. 管理员不得动超管账号（否则可停用超管），也不得调整角色——角色只有超管能给，
//     否则「管理员把自己或他人提成超管」就是一条提权链；
//  3. 超管不能改自己的角色（唯一超管一旦自降，系统再无人可管）。
func ensureAccountChangeAllowed(operator model.UserInfo, target *model.UserInfo, req dto.UserUpdateReq) error {
	wantType := req.Type > 0 && req.Type != target.Type
	wantState := req.State > 0 && req.State != target.State
	if !wantType && !wantState {
		return nil
	}
	if !isAdmin(operator.Type) {
		return newBiz(errcode.CodeForbidden, "无权修改角色或状态")
	}
	if target.Type == model.UserTypeSuperuser && operator.Type != model.UserTypeSuperuser {
		return newBiz(errcode.CodeForbidden, "无权修改超级管理员账号")
	}
	if !wantType {
		return nil
	}
	if operator.Type != model.UserTypeSuperuser {
		return newBiz(errcode.CodeForbidden, "仅超级管理员可调整角色")
	}
	if operator.ID == target.ID {
		return newBiz(errcode.CodeParamError, "不能修改自己的角色")
	}
	if req.Type < model.UserTypeNormal || req.Type > model.UserTypeSuperuser {
		return newBiz(errcode.CodeParamError, "角色取值非法")
	}
	return nil
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
