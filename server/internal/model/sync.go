package model

import "time"

// 同步实体类型（change_log.entity_type / sync ops.type）。
const (
	SyncTypeGroup   = "group"
	SyncTypeAPI     = "api"
	SyncTypeDoc     = "doc"
	SyncTypeExample = "example" // 响应示例（api_return_sample）
)

// 同步操作。
const (
	SyncOpUpsert = "upsert"
	SyncOpDelete = "delete"
)

// Syncable 同步相关列：客户端 uid 与条目版本（= 最近一次 change_log.id）。
// 注意：嵌入到各实体时必须改 uniqueIndex 名（sqlite 全库索引名唯一），
// 故此类型只放字段定义，索引 tag 由各实体自己写。
type Syncable struct {
	ExtUID       string `gorm:"column:ext_uid;size:64" json:"ext_uid"`
	LastChangeID int64  `gorm:"column:last_change_id;not null;default:0" json:"last_change_id"`
}

// ChangeLog 变更流水：只存指针（谁在何时改了哪个条目），项目级游标即 change_log.id。
type ChangeLog struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID  uint64    `gorm:"column:project_id;not null;index" json:"project_id"`
	EntityType string    `gorm:"column:entity_type;size:20;not null" json:"entity_type"`
	ExtUID     string    `gorm:"column:ext_uid;size:64;not null;index" json:"ext_uid"`
	Op         string    `gorm:"column:op;size:10;not null" json:"op"`
	Actor      uint64    `gorm:"column:actor;not null;default:0" json:"actor"`
	Device     string    `gorm:"column:device;size:100;not null;default:''" json:"device"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
}

// TableName 指定表名。
func (ChangeLog) TableName() string { return "change_log" }

// SyncOp push 幂等键：同一 op_id 只处理一次，重复调用返回上次结果。
type SyncOp struct {
	OpID       string    `gorm:"primaryKey;size:64" json:"op_id"`
	ProjectID  uint64    `gorm:"column:project_id;not null;index" json:"project_id"`
	Status     string    `gorm:"column:status;size:20;not null" json:"status"`
	ItemRev    int64     `gorm:"column:item_rev;not null;default:0" json:"item_rev"`
	Reason     string    `gorm:"column:reason;size:200;not null;default:''" json:"reason"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
}

// TableName 指定表名。
func (SyncOp) TableName() string { return "sync_op" }

// UserToken PAT（个人访问令牌）：客户端长期在线用，可删除。
type UserToken struct {
	BaseModel
	UserID    uint64 `gorm:"column:user_id;not null;index" json:"user_id"`
	Name      string `gorm:"column:name;size:100;not null" json:"name"`
	TokenHash string `gorm:"column:token_hash;size:128;not null;uniqueIndex:uk_token_hash" json:"-"`
	// TokenCipher PAT 明文经 AES-256-GCM 加密后的密文（S15）；为空表示旧令牌，无法回显明文。
	TokenCipher string    `gorm:"column:token_cipher;size:512;not null;default:''" json:"-"`
	Scopes      string    `gorm:"column:scopes;size:200;not null;default:''" json:"scopes"`
	ExpireAt    time.Time `gorm:"column:expire_at" json:"expire_at"`
	LastUsed    time.Time `gorm:"column:last_used" json:"last_used"`
	Revoked     int8      `gorm:"column:revoked;not null;default:0" json:"revoked"`
}

// TableName 指定表名。
func (UserToken) TableName() string { return "user_token" }
