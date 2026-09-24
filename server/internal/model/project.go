package model

// Project 对应项目表 project。title 唯一。
type Project struct {
	BaseModel
	Title       string `gorm:"column:title;size:100;not null;uniqueIndex" json:"title"`
	Description string `gorm:"column:description;size:500" json:"description"`
	Version     string `gorm:"column:version;size:100" json:"version"`
	Type        string `gorm:"column:type;size:100" json:"type"`
}

// TableName 指定表名。
func (Project) TableName() string { return "project" }
