package model

// Project 对应项目表 project。
// title 仅在「未删除」项目中唯一（局部唯一索引 uk_project_title），
// 这样软删除后的项目名称可被新项目复用，不会出现列表里看不到却无法新建的情况。
type Project struct {
	BaseModel
	Title       string `gorm:"column:title;size:100;not null;uniqueIndex:uk_project_title,where:is_deleted = 0" json:"title"`
	Description string `gorm:"column:description;size:500" json:"description"`
	Version     string `gorm:"column:version;size:100" json:"version"`
	Type        string `gorm:"column:type;size:100" json:"type"`
}

// TableName 指定表名。
func (Project) TableName() string { return "project" }
