package model

import "time"

// 软删除标记取值。
const (
	IsDeletedNo  int8 = 0 // 正常
	IsDeletedYes int8 = 1 // 已删除
)

// BaseModel 提供各实体通用字段：主键、软删除标记、创建/更新时间。
// 软删除用 is_deleted(0/1)，与旧库及开发规范 §6.3 对齐。
type BaseModel struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	IsDeleted  int8      `gorm:"column:is_deleted;not null;default:0" json:"-"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}
