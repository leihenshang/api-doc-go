package model

import "time"

// 内容快照实体类型与动作（RM-05）。
const (
	SnapshotEntityAPI = "api"
	SnapshotEntityDoc = "doc"

	SnapshotActionCreate  = "create"
	SnapshotActionUpdate  = "update"
	SnapshotActionRestore = "restore"
)

// ContentSnapshot 保存接口/文档每次 Web 保存后的完整载荷，用于字段级 diff 与回滚。
// Payload 为对应 model 的 JSON；只通过带项目读权限的接口暴露。
type ContentSnapshot struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID  uint64    `gorm:"column:project_id;not null;index:idx_snapshot_project" json:"project_id"`
	EntityType string    `gorm:"column:entity_type;size:20;not null;index:idx_snapshot_entity" json:"entity_type"`
	EntityID   uint64    `gorm:"column:entity_id;not null;index:idx_snapshot_entity" json:"entity_id"`
	Revision   int64     `gorm:"column:revision;not null;default:0" json:"revision"`
	ActorID    uint64    `gorm:"column:actor_id;not null;default:0" json:"actor_id"`
	Action     string    `gorm:"column:action;size:20;not null;default:'update'" json:"action"`
	Payload    string    `gorm:"column:payload;type:text;not null" json:"-"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
}

// TableName 指定表名。
func (ContentSnapshot) TableName() string { return "content_snapshot" }
