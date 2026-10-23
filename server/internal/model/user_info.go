package model

import "time"

// 用户类型。与业务约定保持一致，勿随意改数值。
const (
	UserTypeNormal    int8 = 1 // 普通用户
	UserTypeAdmin     int8 = 2 // 管理员
	UserTypeSuperuser int8 = 3 // 超级管理员
)

// 用户状态。
const (
	UserStateNormal   int8 = 1 // 正常
	UserStateDisabled int8 = 2 // 禁用
)

// 界面语言（user_info.lang 取值）。空串表示未选择，由前端按浏览器语言判定。
const (
	LangZhCN = "zh-CN" // 简体中文（默认与兜底）
	LangEnUS = "en-US" // English
)

// IsSupportedLang 判断语言是否在可持久化白名单内。
func IsSupportedLang(lang string) bool {
	return lang == LangZhCN || lang == LangEnUS
}

// UserInfo 对应用户表 user_info。凭证改为 JWT，故不再存 token 字段。
type UserInfo struct {
	BaseModel
	Name string `gorm:"column:name;size:100;not null;uniqueIndex:uk_user_name_active,where:is_deleted = 0" json:"name"`
	Pwd  string `gorm:"column:pwd;size:255;not null" json:"-"`
	// Email 单独唯一（空邮箱不参与），避免同名不同邮箱共存（审核 H-15）。
	Email         string     `gorm:"column:email;size:100;uniqueIndex:uk_user_email_active,where:is_deleted = 0 AND email <> ''" json:"email"`
	Type          int8       `gorm:"column:type;not null;default:1" json:"type"`
	State         int8       `gorm:"column:state;not null;default:1" json:"state"`
	MobileNumber  string     `gorm:"column:mobile_number;size:11" json:"mobile_number"`
	NickName      string     `gorm:"column:nick_name;size:100" json:"nick_name"`
	LastLoginIP   string     `gorm:"column:last_login_ip;size:100" json:"last_login_ip"`
	LastLoginTime *time.Time `gorm:"column:last_login_time" json:"last_login_time"`
	UserFace      string     `gorm:"column:user_face;size:255" json:"user_face"`
	// Lang 界面语言偏好（""=未选择，按浏览器判定；见 LangZhCN / LangEnUS）。登录与 /me 会随用户对象返回。
	Lang string `gorm:"column:lang;size:10;not null;default:''" json:"lang"`
}

// TableName 指定表名。
func (UserInfo) TableName() string { return "user_info" }
