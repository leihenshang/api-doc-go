package model

// 变量作用域：请求级（挂在单个接口）/ 项目级 / 全局（应用级，跨项目共享）。
const (
	VarScopeRequest int8 = 1
	VarScopeProject int8 = 2
	VarScopeGlobal  int8 = 3
)

// 环境「是否项目默认」标记。
const (
	VarEnvNotDefault int8 = 0
	VarEnvDefault    int8 = 1
)

// 变量「是否启用」标记（停用的变量在解析时视为不存在）。
const (
	VarDisabled int8 = 0
	VarEnabled  int8 = 1
)

// 变量「是否敏感」标记：仅表示展示与接口回传脱敏，值本身明文入库，不是安全边界。
const (
	VarNotSecret int8 = 0
	VarSecret    int8 = 1
)

// VariableEnv 对应环境表 variable_env：项目级多环境（dev/test/prod）。
// 环境只影响项目变量的取值与展示，不改写任何接口数据。
type VariableEnv struct {
	BaseModel
	ProjectID   uint64 `gorm:"column:project_id;not null;index" json:"project_id"`
	Name        string `gorm:"column:name;size:100;not null" json:"name"`
	Description string `gorm:"column:description;type:text" json:"description"`
	Color       string `gorm:"column:color;size:20" json:"color"`
	// IsDefault 项目默认环境：同项目由 service 层保证唯一。
	IsDefault int8 `gorm:"column:is_default;not null;default:0" json:"is_default"`
	Priority  int  `gorm:"column:priority;not null;default:0" json:"priority"`
}

// TableName 指定表名。
func (VariableEnv) TableName() string { return "variable_env" }

// Variable 对应变量表 variable：请求级 / 项目级 / 全局共用一张表，靠 scope 区分。
// 解析优先级：请求级 > 项目级(当前环境) > 项目级(通用 env_id=0) > 全局。
// 同层同环境不允许重名（局部唯一索引 uk_variable_key，排除软删除行）。
type Variable struct {
	BaseModel
	Scope int8 `gorm:"column:scope;not null;default:1;uniqueIndex:uk_variable_key,priority:1,where:is_deleted = 0" json:"scope"`
	// ProjectID 请求级/项目级必填；全局变量固定 0。
	ProjectID uint64 `gorm:"column:project_id;not null;default:0;uniqueIndex:uk_variable_key,priority:2;index:idx_variable_scope,priority:1" json:"project_id"`
	// ApiID 仅请求级（scope=1）填接口 id，其余为 0。
	ApiID uint64 `gorm:"column:api_id;not null;default:0;uniqueIndex:uk_variable_key,priority:3;index:idx_variable_scope,priority:2" json:"api_id"`
	// EnvID 仅项目级生效：0 = 通用兜底，>0 = 绑定某环境；全局变量固定 0。
	EnvID uint64 `gorm:"column:env_id;not null;default:0;uniqueIndex:uk_variable_key,priority:4" json:"env_id"`
	Name  string `gorm:"column:name;size:100;not null;uniqueIndex:uk_variable_key,priority:5;index:idx_variable_name" json:"name"`
	// Value 变量值（明文，含敏感值；对外接口对敏感值脱敏）。
	Value       string `gorm:"column:value;type:text" json:"value"`
	Description string `gorm:"column:description;type:text" json:"description"`
	// IsSecret 1 = 敏感值：列表与解析结果只回掩码。
	IsSecret int8 `gorm:"column:is_secret;not null;default:0" json:"is_secret"`
	Enabled  int8 `gorm:"column:enabled;not null;default:1" json:"enabled"`
	Priority int  `gorm:"column:priority;not null;default:0" json:"priority"`
}

// TableName 指定表名。
func (Variable) TableName() string { return "variable" }
