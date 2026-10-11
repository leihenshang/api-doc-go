package model

// ApiReturnSample 接口的多个响应示例（客户端 ResponseExample 落库形态）。
// 与 api.http_return_sample 不同：后者是接口的「主/默认样例」（用于 mock、代码生成、
// 详情页默认展示），本表承载用户保存的多条命名响应示例，可在 Web 与客户端间同步。
type ApiReturnSample struct {
	BaseModel
	ProjectID    uint64 `gorm:"column:project_id;not null;index" json:"project_id"`
	ApiID        uint64 `gorm:"column:api_id;not null;index" json:"api_id"`
	ExtUID       string `gorm:"column:ext_uid;size:64;uniqueIndex:uk_api_sample_ext_uid,where:is_deleted = 0 AND ext_uid != ''" json:"ext_uid"`
	LastChangeID int64  `gorm:"column:last_change_id;not null;default:0" json:"last_change_id"`
	Name         string `gorm:"column:name;size:500;not null" json:"name"`
	// SampleText 响应正文（JSON 文本，与 http_return_sample 同口径）。
	SampleText string `gorm:"column:sample_text;type:text" json:"sample_text"`
	Status     int    `gorm:"column:status;not null;default:200" json:"status"`
	// ContentType 响应 Content-Type，缺省时按 JSON 处理。
	ContentType string `gorm:"column:content_type;size:200;not null;default:''" json:"content_type"`
	Seq         int    `gorm:"column:seq;not null;default:0" json:"seq"`
}

// TableName 指定表名。
func (ApiReturnSample) TableName() string { return "api_return_sample" }
