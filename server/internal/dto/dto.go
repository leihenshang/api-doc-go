// Package dto 请求/响应传输结构体。按操作拆分字段，用 binding tag 声明校验。
package dto

import "encoding/json"

// ---- 用户 ----

// LoginReq 登录入参。
type LoginReq struct {
	Name string `json:"name" binding:"required"`
	Pwd  string `json:"pwd" binding:"required"`
}

// RegisterReq 注册入参。
type RegisterReq struct {
	Name     string `json:"name" binding:"required"`
	Pwd      string `json:"pwd" binding:"required"`
	RePwd    string `json:"re_pwd" binding:"required"`
	NickName string `json:"nick_name"`
	Email    string `json:"email"`
}

// UserUpdateReq 修改资料入参。
type UserUpdateReq struct {
	NickName string `json:"nick_name"`
	Email    string `json:"email"`
	UserFace string `json:"user_face"`
	State    int8   `json:"state"`
	Type     int8   `json:"type"`
}

// UpdatePwdReq 改密入参。
type UpdatePwdReq struct {
	OldPwd string `json:"old_pwd" binding:"required"`
	Pwd    string `json:"pwd" binding:"required"`
	RePwd  string `json:"re_pwd" binding:"required"`
}

// CheckRepeatReq 查重入参。
type CheckRepeatReq struct {
	Keyword string `form:"keyword" binding:"required"`
}

// ---- 项目 / 成员 ----

// ProjectCreateReq 创建项目。
type ProjectCreateReq struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Type        string `json:"type"`
}

// ProjectUpdateReq 更新项目。
type ProjectUpdateReq struct {
	ID          uint64 `json:"id" binding:"required"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Type        string `json:"type"`
}

// MemberAddReq 添加成员。
type MemberAddReq struct {
	ProjectID  uint64 `json:"project_id" binding:"required"`
	UserID     uint64 `json:"user_id" binding:"required"`
	Permission int8   `json:"permission"`
}

// SetPermissionReq 设置成员权限。
type SetPermissionReq struct {
	ProjectID  uint64 `json:"project_id" binding:"required"`
	UserID     uint64 `json:"user_id" binding:"required"`
	Permission int8   `json:"permission"`
}

// SetLeaderReq 设置负责人。
type SetLeaderReq struct {
	ProjectID uint64 `json:"project_id" binding:"required"`
	UserID    uint64 `json:"user_id" binding:"required"`
}

// QuitProjectReq 退出项目。
type QuitProjectReq struct {
	ProjectID uint64 `json:"project_id" binding:"required"`
}

// IDReq 仅含主键的通用入参。
type IDReq struct {
	ID uint64 `json:"id" form:"id" binding:"required"`
}

// ---- 分组 ----

// GroupCreateReq 创建分组。
type GroupCreateReq struct {
	ProjectID uint64 `json:"project_id" binding:"required"`
	PID       uint64 `json:"p_id"`
	Title     string `json:"title" binding:"required"`
	Type      int8   `json:"type"`
	Priority  int    `json:"priority"`
}

// GroupUpdateReq 更新分组。
type GroupUpdateReq struct {
	ID       uint64 `json:"id" binding:"required"`
	Title    string `json:"title"`
	PID      uint64 `json:"p_id"`
	Priority int    `json:"priority"`
}

// ---- 接口 ----

// ApiCreateReq 创建接口文档。结构化子数据以原始 JSON 传入，存为文本列。
type ApiCreateReq struct {
	ProjectID         uint64          `json:"project_id" binding:"required"`
	GroupID           uint64          `json:"group_id"`
	GroupIDSecond     uint64          `json:"group_id_second"`
	ApiName           string          `json:"api_name" binding:"required"`
	ProtocolType      string          `json:"protocol_type"`
	HttpMethodType    string          `json:"http_method_type"`
	URL               string          `json:"url"`
	Description       string          `json:"description"`
	ObjectName        string          `json:"object_name"`
	FunctionName      string          `json:"function_name"`
	DevelopLanguage   string          `json:"develop_language"`
	HttpRequestHeader json.RawMessage `json:"http_request_header"`
	HttpRequestParams json.RawMessage `json:"http_request_params"`
	HttpReturnType    string          `json:"http_return_type"`
	HttpReturnParams  json.RawMessage `json:"http_return_params"`
	HttpReturnSample  json.RawMessage `json:"http_return_sample"`
}

// ApiUpdateReq 更新接口文档。
type ApiUpdateReq struct {
	ID uint64 `json:"id" binding:"required"`
	ApiCreateReq
}

// ApiListQuery 接口列表查询参数。
type ApiListQuery struct {
	ProjectID uint64 `form:"project_id" binding:"required"`
	GroupID   uint64 `form:"group_id"`
	Deleted   int8   `form:"deleted"`
	Keyword   string `form:"keyword"`
	PageNum   int    `form:"page_num"`
	PageSize  int    `form:"page_size"`
}

// ---- 文档 ----

// DocCreateReq 创建文档。
type DocCreateReq struct {
	ProjectID uint64 `json:"project_id" binding:"required"`
	GroupID   uint64 `json:"group_id"`
	Title     string `json:"title" binding:"required"`
	Content   string `json:"content"`
}

// DocUpdateReq 更新文档。
type DocUpdateReq struct {
	ID      uint64 `json:"id" binding:"required"`
	Title   string `json:"title"`
	Content string `json:"content"`
	GroupID uint64 `json:"group_id"`
	IsTop   int8   `json:"is_top"`
}

// ---- Bug ----

// BugCreateReq 创建 Bug。
type BugCreateReq struct {
	ProjectID uint64 `json:"project_id" binding:"required"`
	Title     string `json:"title" binding:"required"`
	Content   string `json:"content" binding:"required"`
	Level     int8   `json:"level"`
	Comment   string `json:"comment"`
}

// BugUpdateReq 更新 Bug。
type BugUpdateReq struct {
	ID      uint64 `json:"id" binding:"required"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Level   int8   `json:"level"`
	Status  int8   `json:"status"`
	Comment string `json:"comment"`
}

// BugAssignReq 指派 Bug。
type BugAssignReq struct {
	BugID    uint64 `json:"bug_id" binding:"required"`
	ToUserID uint64 `json:"to_user_id" binding:"required"`
	Comment  string `json:"comment"`
}

// ---- 字段映射 ----

// FieldMappingReq 字段映射创建/更新。
type FieldMappingReq struct {
	ID          uint64 `json:"id"`
	ProjectID   uint64 `json:"project_id" binding:"required"`
	Field       string `json:"field" binding:"required"`
	Type        string `json:"type" binding:"required"`
	Description string `json:"description"`
}

// ---- 属性 / 消息 / 通知 ----

// PropertyReq 公共属性。
type PropertyReq struct {
	ID          uint64 `json:"id"`
	Tag         string `json:"tag" binding:"required"`
	TagName     string `json:"tag_name" binding:"required"`
	Description string `json:"description"`
}

// MessageSendReq 验证码发送。
type MessageSendReq struct {
	ReceiveSource string `json:"receive_source" binding:"required"`
	SendType      int8   `json:"send_type"`
}

// ---- 外部数据库连接 ----

// DatabaseReq 连接创建/更新。更新时 Password 可留空表示沿用原口令。
type DatabaseReq struct {
	ID           uint64 `json:"id"`
	ProjectID    uint64 `json:"project_id" binding:"required"`
	Address      string `json:"address" binding:"required"`
	Port         int    `json:"port" binding:"required"`
	Username     string `json:"username" binding:"required"`
	Password     string `json:"password"`
	DatabaseName string `json:"database_name" binding:"required"`
	Type         int8   `json:"type"`
	Description  string `json:"description"`
}
