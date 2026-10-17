package model

// UserProject 对应用户-项目成员关系表 user_project。
// 细粒度 RBAC 权限在 P2 落地，此处仅表达成员关系与负责人标记。
type UserProject struct {
	BaseModel
	UserID uint64 `gorm:"column:user_id;not null;index:idx_user_project;uniqueIndex:uk_user_project,where:is_deleted = 0" json:"user_id"`
	// ProjectID 与 UserID 组成 (project_id, user_id) 部分唯一索引：
	// 同一项目里同一用户只能有一行活数据，否则并发添加会各插一行、权限判定结果不可预测（P1-9）。
	// where 子句必须与迁移 0016 的 WHERE is_deleted = 0 一致，软删行不占位。
	ProjectID  uint64 `gorm:"column:project_id;not null;index:idx_user_project;uniqueIndex:uk_user_project,where:is_deleted = 0" json:"project_id"`
	IsLeader   int8   `gorm:"column:is_leader;not null;default:0" json:"is_leader"`
	Permission int8   `gorm:"column:permission;not null;default:1" json:"permission"`
}

// TableName 指定表名。
func (UserProject) TableName() string { return "user_project" }
