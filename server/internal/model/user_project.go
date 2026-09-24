package model

// UserProject 对应用户-项目成员关系表 user_project。
// 细粒度 RBAC 权限在 P2 落地，此处仅表达成员关系与负责人标记。
type UserProject struct {
	BaseModel
	UserID     uint64 `gorm:"column:user_id;not null;index:idx_user_project" json:"user_id"`
	ProjectID  uint64 `gorm:"column:project_id;not null;index:idx_user_project" json:"project_id"`
	IsLeader   int8   `gorm:"column:is_leader;not null;default:0" json:"is_leader"`
	Permission int8   `gorm:"column:permission;not null;default:1" json:"permission"`
}

// TableName 指定表名。
func (UserProject) TableName() string { return "user_project" }
