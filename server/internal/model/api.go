package model

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
	Source            string `gorm:"column:source;size:100;not null;default:''" json:"source"`
	HttpRequestHeader string `gorm:"column:http_request_header;type:text" json:"http_request_header"`
	HttpRequestParams string `gorm:"column:http_request_params;type:text" json:"http_request_params"`
	HttpReturnType    string `gorm:"column:http_return_type;size:1000" json:"http_return_type"`
	HttpReturnParams  string `gorm:"column:http_return_params;type:text" json:"http_return_params"`
	HttpReturnSample  string `gorm:"column:http_return_sample;type:text" json:"http_return_sample"`
}

// TableName 指定表名。
func (Api) TableName() string { return "api" }
