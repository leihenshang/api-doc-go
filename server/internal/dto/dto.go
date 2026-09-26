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
// Lang 为界面语言（zh-CN / en-US），本人即可修改，空串表示不改。
type UserUpdateReq struct {
	NickName string `json:"nick_name"`
	Email    string `json:"email"`
	UserFace string `json:"user_face"`
	Lang     string `json:"lang"`
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
// ExpectedUpdateTime 可选：客户端载入时的 update_time（RFC3339）。传入则做乐观锁校验，
// 与他人保存冲突时返回 14；不传则保持历史行为（整体覆盖，不做校验）。
type ApiUpdateReq struct {
	ID                 uint64 `json:"id" binding:"required"`
	ExpectedUpdateTime string `json:"expected_update_time"`
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

// ---- 拖拽排序 ----

// SortItem 排序项：ParentID 为分组树父级（分组=p_id，接口/文档=group_id），Priority 越小越靠前。
type SortItem struct {
	ID       uint64 `json:"id" binding:"required"`
	ParentID uint64 `json:"parent_id"`
	Priority int    `json:"priority"`
}

// SortReq 批量保存拖拽后的顺序；Kind 取 group / api / doc。
type SortReq struct {
	ProjectID uint64     `json:"project_id" binding:"required"`
	Kind      string     `json:"kind" binding:"required,oneof=group api doc"`
	Items     []SortItem `json:"items" binding:"required,min=1"`
}

// ---- 文档 ----

// DocCreateReq 创建文档。
type DocCreateReq struct {
	ProjectID uint64 `json:"project_id" binding:"required"`
	GroupID   uint64 `json:"group_id"`
	Title     string `json:"title" binding:"required"`
	Content   string `json:"content"`
	// Icon 文档图标：上传图片的 URL 或单个 emoji。
	Icon string `json:"icon"`
}

// DocUpdateReq 更新文档。
type DocUpdateReq struct {
	ID      uint64 `json:"id" binding:"required"`
	Title   string `json:"title"`
	Content string `json:"content"`
	GroupID uint64 `json:"group_id"`
	IsTop   int8   `json:"is_top"`
	Icon    string `json:"icon"`
	// ExpectedUpdateTime 可选：载入时的 update_time，传入则做乐观锁校验（冲突返回 14）。
	ExpectedUpdateTime string `json:"expected_update_time"`
}

// ---- 分享链接 ----

// ShareLinkReq 创建分享链接：expire_days > 0 时按天数设置有效期，0 表示永久有效。
type ShareLinkReq struct {
	ProjectID  uint64 `json:"project_id" binding:"required"`
	Remark     string `json:"remark"`
	ExpireDays int    `json:"expire_days"`
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
	// IsTop 为空表示不改动置顶状态（0 取消置顶 / 1 置顶）。
	IsTop *int8 `json:"is_top"`
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

// IDsReq 批量操作请求（回收站批量恢复/彻底删除）。
type IDsReq struct {
	IDs []uint64 `json:"ids" binding:"required,min=1,dive,gt=0"`
}

// ---- 变量 / 环境 ----

// VariableEnvReq 环境创建/更新。
type VariableEnvReq struct {
	ID          uint64 `json:"id"`
	ProjectID   uint64 `json:"project_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Priority    int    `json:"priority"`
}

// VariableReq 变量创建/更新：Scope 1 请求级 / 2 项目级 / 3 全局（应用级）。
// Value 为 nil 表示不修改原值；Enabled 为 nil 表示新建默认启用、更新保持不变。
// 项目级变量的 EnvID 以请求值为准（0 = 通用变量）。
type VariableReq struct {
	ID          uint64  `json:"id"`
	Scope       int8    `json:"scope" binding:"required"`
	ProjectID   uint64  `json:"project_id"`
	ApiID       uint64  `json:"api_id"`
	EnvID       uint64  `json:"env_id"`
	Name        string  `json:"name" binding:"required"`
	Value       *string `json:"value"`
	Description string  `json:"description"`
	IsSecret    int8    `json:"is_secret"`
	Enabled     *int8   `json:"enabled"`
	Priority    int     `json:"priority"`
	// ExpectedUpdateTime 可选：载入时的 update_time，传入则做乐观锁校验（冲突返回 14）。
	ExpectedUpdateTime string `json:"expected_update_time"`
}

// VariableListQuery 变量列表查询：EnvID 缺省 = 不筛选，0 = 仅通用变量；Scope 0 = 全部。
type VariableListQuery struct {
	ProjectID uint64  `form:"project_id"`
	Scope     int8    `form:"scope"`
	ApiID     uint64  `form:"api_id"`
	EnvID     *uint64 `form:"env_id"`
	Keyword   string  `form:"keyword"`
}

// VariableSummaryQuery 列表徽标批量查询：api_ids 为逗号分隔的接口 id（单次上限 200）。
type VariableSummaryQuery struct {
	ProjectID uint64 `form:"project_id" binding:"required"`
	EnvID     uint64 `form:"env_id"`
	ApiIDs    string `form:"api_ids" binding:"required"`
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

// ---- 接口导入（Bruno） ----

// BrunoFile 集合内的单个文本文件：相对集合根目录的路径 + 内容。
type BrunoFile struct {
	Path    string `json:"path" binding:"required"`
	Content string `json:"content"`
}

// BrunoImportReq Bruno 集合导入入参：前端把集合目录内的 yml 文件整体上传。
type BrunoImportReq struct {
	Files []BrunoFile `json:"files" binding:"required,min=1,dive"`
}
