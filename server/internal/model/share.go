package model

import "time"

// ShareLink 对应分享链接表 share_link：为项目生成**免登录只读**访问入口。
// 令牌随机生成、可设有效期；删除即失效（软删除）。
type ShareLink struct {
	BaseModel
	ProjectID uint64 `gorm:"column:project_id;not null;index" json:"project_id"`
	// UserID 创建者（便于审计）。
	UserID uint64 `gorm:"column:user_id;not null;default:0" json:"user_id"`
	// Token 访问令牌（随机、全局唯一；局部唯一索引排除软删除行）。
	Token string `gorm:"column:token;size:64;not null;uniqueIndex:uk_share_token,where:is_deleted = 0" json:"token"`
	// Remark 备注（给谁看的、用途）。
	Remark string `gorm:"column:remark;size:200" json:"remark"`
	// ExpireTime 失效时间，空表示永久有效。
	ExpireTime *time.Time `gorm:"column:expire_time" json:"expire_time"`
}

// TableName 指定表名。
func (ShareLink) TableName() string { return "share_link" }
