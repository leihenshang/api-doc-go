package model

import "time"

// 接口生命周期状态（RM-04）：0 视为历史数据/未设置，按已发布处理。
const (
	ApiStatusDraft      int8 = 1
	ApiStatusPublished  int8 = 2
	ApiStatusDeprecated int8 = 3
)

// Api 对应接口文档表 api。请求/返回相关的结构化数据以 JSON 文本列存储，
// 方言无关（sqlite 存 text、postgres 同样可用），解析在 service 层完成。
type Api struct {
	BaseModel
	ProjectID       uint64 `gorm:"column:project_id;not null;index" json:"project_id"`
	ExtUID          string `gorm:"column:ext_uid;size:64;uniqueIndex:uk_api_ext_uid,where:is_deleted = 0 AND ext_uid != ''" json:"ext_uid"`
	LastChangeID    int64  `gorm:"column:last_change_id;not null;default:0" json:"last_change_id"`
	GroupID         uint64 `gorm:"column:group_id;not null;default:0;index" json:"group_id"`
	GroupIDSecond   uint64 `gorm:"column:group_id_second;not null;default:0" json:"group_id_second"`
	Priority        int    `gorm:"column:priority;not null;default:0" json:"priority"`
	Description     string `gorm:"column:description;type:text" json:"description"`
	ProtocolType    string `gorm:"column:protocol_type;size:100" json:"protocol_type"`
	HttpMethodType  string `gorm:"column:http_method_type;size:100" json:"http_method_type"`
	URL             string `gorm:"column:url;size:1000" json:"url"`
	ApiName         string `gorm:"column:api_name;size:500" json:"api_name"`
	ObjectName      string `gorm:"column:object_name;size:100" json:"object_name"`
	FunctionName    string `gorm:"column:function_name;size:100" json:"function_name"`
	DevelopLanguage string `gorm:"column:develop_language;size:100" json:"develop_language"`
	// Source 记录接口来源：openapi-import / bruno-import，空表示手工创建。
	Source string `gorm:"column:source;size:100;not null;default:''" json:"source"`
	// Status 生命周期：1=草稿 2=已发布 3=已废弃；Version 为业务版本号（可空）。
	// DeprecatedAt 在首次进入废弃状态时由 service 填值，回到其它状态会清空。
	Status            int8       `gorm:"column:status;not null;default:2;index" json:"status"`
	Version           string     `gorm:"column:version;size:50;not null;default:''" json:"version"`
	DeprecatedAt      *time.Time `gorm:"column:deprecated_at" json:"deprecated_at,omitempty"`
	HttpRequestHeader string     `gorm:"column:http_request_header;type:text" json:"http_request_header"`
	HttpRequestParams string     `gorm:"column:http_request_params;type:text" json:"http_request_params"`
	HttpReturnType    string     `gorm:"column:http_return_type;size:1000" json:"http_return_type"`
	HttpReturnParams  string     `gorm:"column:http_return_params;type:text" json:"http_return_params"`
	HttpReturnSample  string     `gorm:"column:http_return_sample;type:text" json:"http_return_sample"`
	// —— 客户端富字段（S16 扩展）：以原始 JSON/文本列存储，服务端不解释语义、原样回传客户端；
	// Web 详情按字段解析展示。键集与客户端 collection/sync_extra_fields.go 逐字一致。
	HttpRequestBodyType   string `gorm:"column:http_request_body_type;size:100" json:"http_request_body_type"`
	HttpRequestBody       string `gorm:"column:http_request_body;type:text" json:"http_request_body"`
	HttpRequestBodyForm   string `gorm:"column:http_request_body_form;type:text" json:"http_request_body_form"`
	HttpRequestBodyDescs  string `gorm:"column:http_request_body_descs;type:text" json:"http_request_body_descs"`
	HttpAuth              string `gorm:"column:http_auth;type:text" json:"http_auth"`
	RequestSettings       string `gorm:"column:request_settings;type:text" json:"request_settings"`
	HttpScript            string `gorm:"column:http_script;type:text" json:"http_script"`
	ScriptVars            string `gorm:"column:script_vars;type:text" json:"script_vars"`
	ScriptAsserts         string `gorm:"column:script_asserts;type:text" json:"script_asserts"`
	HttpReturnContentType string `gorm:"column:http_return_content_type;size:100" json:"http_return_content_type"`
}

// TableName 指定表名。
func (Api) TableName() string { return "api" }
