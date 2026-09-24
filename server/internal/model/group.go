package model

// 分组类型。与旧库对齐。
const (
	GroupTypeUndefined int8 = 0
	GroupTypeAPI       int8 = 1
	GroupTypeProject   int8 = 2
	GroupTypeDoc       int8 = 3
)

// Group 对应分组表 group，支持多级（p_id 指向父分组，0 为根）。
type Group struct {
	BaseModel
	PID       uint64 `gorm:"column:p_id;not null;default:0;index" json:"p_id"`
	ProjectID uint64 `gorm:"column:project_id;not null;default:0;index" json:"project_id"`
	Title     string `gorm:"column:title;size:100;not null" json:"title"`
	Priority  int    `gorm:"column:priority;not null;default:0" json:"priority"`
	Type      int8   `gorm:"column:type;not null;default:0" json:"type"`
}

// TableName 指定表名。
func (Group) TableName() string { return "group" }
