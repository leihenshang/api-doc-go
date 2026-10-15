package model

import "time"

// 文档状态。
const (
	DocStateNormal   int8 = 0
	DocStateReview   int8 = 1
	DocStateDisabled int8 = 2
)

// Doc 对应项目文档表 doc（Markdown 内容）。
type Doc struct {
	BaseModel
	UserID       uint64 `gorm:"column:user_id;not null;index" json:"user_id"`
	ProjectID    uint64 `gorm:"column:project_id;not null;index" json:"project_id"`
	GroupID      uint64 `gorm:"column:group_id;not null;default:0;index" json:"group_id"`
	ExtUID       string `gorm:"column:ext_uid;size:64;uniqueIndex:uk_doc_ext_uid,where:is_deleted = 0 AND ext_uid != ''" json:"ext_uid"`
	LastChangeID int64  `gorm:"column:last_change_id;not null;default:0" json:"last_change_id"`
	Title        string `gorm:"column:title;size:100;not null" json:"title"`
	Content      string `gorm:"column:content;type:text;not null" json:"content"`
	// Icon 文档图标：上传图片的 URL 或单个 emoji（空则前端用默认文档图标）。
	Icon      string `gorm:"column:icon;size:500" json:"icon"`
	State     int8   `gorm:"column:state;not null;default:0" json:"state"`
	ViewCount int    `gorm:"column:view_count;not null;default:0" json:"view_count"`
	LikeCount int    `gorm:"column:like_count;not null;default:0" json:"like_count"`
	IsTop     int8   `gorm:"column:is_top;not null;default:0" json:"is_top"`
	Priority  int    `gorm:"column:priority;not null;default:0" json:"priority"`
}

// TableName 指定表名。
func (Doc) TableName() string { return "doc" }

// Bug 状态与等级枚举。
const (
	BugStatusPending  int8 = 1
	BugStatusResolved int8 = 2
	BugStatusIgnore   int8 = 3
)

// Bug 对应缺陷表 bug。
type Bug struct {
	BaseModel
	ProjectID uint64 `gorm:"column:project_id;not null;index" json:"project_id"`
	ToUserID  uint64 `gorm:"column:to_user_id;not null;default:0" json:"to_user_id"`
	Title     string `gorm:"column:title;size:100;not null" json:"title"`
	Content   string `gorm:"column:content;type:text;not null" json:"content"`
	Comment   string `gorm:"column:comment;size:1000" json:"comment"`
	Status    int8   `gorm:"column:status;not null;default:1" json:"status"`
	Level     int8   `gorm:"column:level;not null;default:1" json:"level"`
	// IsTop 备忘置顶（1 置顶 / 0 普通），列表按 is_top 优先排序。
	IsTop int8 `gorm:"column:is_top;not null;default:0" json:"is_top"`
}

// TableName 指定表名。
func (Bug) TableName() string { return "bug" }

// BugAssign 对应缺陷指派记录表 bug_assign。
type BugAssign struct {
	BaseModel
	BugID      uint64 `gorm:"column:bug_id;not null;index" json:"bug_id"`
	FromUserID uint64 `gorm:"column:from_user_id;not null;default:0" json:"from_user_id"`
	ToUserID   uint64 `gorm:"column:to_user_id;not null;default:0" json:"to_user_id"`
	Comment    string `gorm:"column:comment;size:1000" json:"comment"`
	Status     int8   `gorm:"column:status;not null;default:1" json:"status"`
}

// TableName 指定表名。
func (BugAssign) TableName() string { return "bug_assign" }

// FieldMapping 对应字段映射表 field_mapping。
type FieldMapping struct {
	BaseModel
	ProjectID   uint64 `gorm:"column:project_id;not null;index" json:"project_id"`
	Field       string `gorm:"column:field;size:100;not null" json:"field"`
	Type        string `gorm:"column:type;size:100;not null" json:"type"`
	Description string `gorm:"column:description;size:100" json:"description"`
}

// TableName 指定表名。
func (FieldMapping) TableName() string { return "field_mapping" }

// Property 对应公共属性表 property。
type Property struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	Tag         string `gorm:"column:tag;size:100;not null" json:"tag"`
	TagName     string `gorm:"column:tag_name;size:100;not null" json:"tag_name"`
	Description string `gorm:"column:description;size:100" json:"description"`
}

// TableName 指定表名。
func (Property) TableName() string { return "property" }

// Message 对应消息/验证码表 message。
type Message struct {
	BaseModel
	Content       string     `gorm:"column:content;size:500" json:"content"`
	SendType      int8       `gorm:"column:send_type;not null;default:0" json:"send_type"`
	ReceiveSource string     `gorm:"column:receive_source;size:100;not null" json:"receive_source"`
	Title         string     `gorm:"column:title;size:100" json:"title"`
	ExpireTime    time.Time  `gorm:"column:expire_time;not null" json:"expire_time"`
	UsedTime      *time.Time `gorm:"column:used_time" json:"used_time"`
	IsUsed        int8       `gorm:"column:is_used;not null;default:0" json:"is_used"`
	Code          string     `gorm:"column:code;size:100" json:"code"`
}

// TableName 指定表名。
func (Message) TableName() string { return "message" }

// Notification 对应站内通知表 notification。
type Notification struct {
	BaseModel
	Title    string `gorm:"column:title;size:100" json:"title"`
	Content  string `gorm:"column:content;size:500" json:"content"`
	SendType int8   `gorm:"column:send_type;not null;default:0" json:"send_type"`
	Receiver uint64 `gorm:"column:receiver;not null;index" json:"receiver"`
	IsRead   int8   `gorm:"column:is_read;not null;default:0" json:"read"`
}

// TableName 指定表名。
func (Notification) TableName() string { return "notification" }

// FileRecord 对应文件表 file。
type FileRecord struct {
	BaseModel
	Title string `gorm:"column:title;size:200;not null" json:"title"`
	Path  string `gorm:"column:path;size:500" json:"path"`
	// Hash 内容寻址（sha256 hex，S13）：离线文档图片映射用。
	Hash string `gorm:"column:hash;size:64;index" json:"hash"`
	Size int64  `gorm:"column:size;not null;default:0" json:"size"`
	Mime string `gorm:"column:mime;size:100;not null;default:''" json:"mime"`
}

// TableName 指定表名。
func (FileRecord) TableName() string { return "file" }

// 操作日志对象类型。
const (
	LogObjectProject int8 = 1
	LogObjectGroup   int8 = 2
	LogObjectAPI     int8 = 3
	LogObjectUser    int8 = 4
	LogObjectDoc     int8 = 5
	// LogObjectVariable 变量（请求级/项目级/全局共用）。
	LogObjectVariable int8 = 6
	// LogObjectVariableEnv 项目级环境。
	LogObjectVariableEnv int8 = 7
)

// OperationLog 对应操作日志表 operation_log。
type OperationLog struct {
	BaseModel
	UserID    uint64 `gorm:"column:user_id;not null;index" json:"user_id"`
	Action    string `gorm:"column:action;size:100;not null" json:"action"`
	Content   string `gorm:"column:content;size:500" json:"content"`
	ObjectID  uint64 `gorm:"column:object_id;not null;default:0;index" json:"object_id"`
	Type      int8   `gorm:"column:type;not null;default:0" json:"type"`
	ProjectID uint64 `gorm:"column:project_id;not null;index" json:"project_id"`
}

// TableName 指定表名。
func (OperationLog) TableName() string { return "operation_log" }
