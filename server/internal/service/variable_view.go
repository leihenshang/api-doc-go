package service

// 变量的视图与掩码（P2-14：从 variable.go 拆出）。
// 「能看什么」与「怎么显示」在这里；写权限判定在 variable.go。

import (
	"strings"

	"apidoc/internal/model"

	"github.com/leihenshang/api-doc-go-share/varx"
)

// varRank 返回变量在当前环境下的解析优先级（越小越优先）；ok=false 表示不参与解析。
// 优先级表在共享包 varx，本层只负责 model → varx.Var 的映射。
func varRank(v model.Variable, envID uint64) (int, bool) {
	return varx.Rank(toShareVar(v), envID)
}

// toShareVar 把实体映射为共享层变量：项目变量按是否绑定环境区分「当前环境」与「通用」。
func toShareVar(v model.Variable) varx.Var {
	scope := varx.ScopeGlobal
	switch v.Scope {
	case model.VarScopeRequest:
		scope = varx.ScopeRequest
	case model.VarScopeProject:
		scope = varx.ScopeProjectEnv
		if v.EnvID == 0 {
			scope = varx.ScopeProjectCommon
		}
	}
	return varx.Var{
		Name:    v.Name,
		Value:   v.Value,
		Scope:   scope,
		EnvID:   v.EnvID,
		Enabled: v.Enabled == model.VarEnabled,
		Secret:  v.IsSecret == model.VarSecret,
	}
}

// varSourceOf 返回变量来源标记。
func varSourceOf(v model.Variable) string {
	switch v.Scope {
	case model.VarScopeRequest:
		return varSourceRequest
	case model.VarScopeProject:
		return varSourceProject
	}
	return varSourceGlobal
}

// scopeLabel 生成唯一冲突文案里的作用域描述。
func scopeLabel(v *model.Variable) string {
	switch v.Scope {
	case model.VarScopeRequest:
		return "该接口下"
	case model.VarScopeProject:
		if v.EnvID == 0 {
			return "该项目的通用变量中"
		}
		return "该项目环境中"
	}
	return "全局变量中"
}

// maskVariable 实体转视图：敏感值只回掩码并标记是否有值。
func maskVariable(v model.Variable) VarView {
	view := plainVariable(v)
	view.HasValue = v.Value != ""
	// 掩码规则（敏感且有值才掩码）与共享包一致，避免两端各写一套判断。
	view.Value = varx.MaskedValueOf(toShareVar(v))
	return view
}

// plainVariable 实体转视图（值不脱敏，仅用于有写权限的回显路径）。
func plainVariable(v model.Variable) VarView {
	return VarView{
		ID: v.ID, Scope: v.Scope, ProjectID: v.ProjectID, ApiID: v.ApiID, EnvID: v.EnvID,
		Name: v.Name, Value: v.Value, Description: v.Description, IsSecret: v.IsSecret,
		HasValue: v.Value != "", Enabled: v.Enabled, Priority: v.Priority,
		CreateTime: v.CreateTime, UpdateTime: v.UpdateTime,
	}
}

// maskVariables 批量转视图。
func maskVariables(items []model.Variable) []VarView {
	out := make([]VarView, 0, len(items))
	for _, v := range items {
		out = append(out, maskVariable(v))
	}
	return out
}

// derefString 取可选字符串值，nil 视为空串。
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// joinNames 列出前 limit 个名称，超出时追加「等 N 个」。
func joinNames(items []model.Variable, limit int) string {
	names := make([]string, 0, len(items))
	for _, v := range items {
		names = append(names, v.Name)
	}
	if len(names) <= limit {
		return strings.Join(names, "、")
	}
	return strings.Join(names[:limit], "、") + " 等 " + itoa(len(names)) + " 个"
}

// collectVarNames 提取文本中引用的变量名（去重保序，不含内置变量）。
func collectVarNames(text string) []string {
	return varx.Names(text)
}

// apiVarNames 汇总接口在 URL / 请求头 value / 请求参数示例中引用的变量名。
func apiVarNames(a *model.Api) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	add := func(names []string) {
		for _, name := range names {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	add(collectVarNames(a.URL))
	for _, h := range parseRows(a.HttpRequestHeader) {
		if value, ok := h["value"].(string); ok {
			add(collectVarNames(value))
		}
	}
	for _, p := range parseRows(a.HttpRequestParams) {
		if example, ok := p["example"].(string); ok {
			add(collectVarNames(example))
		}
	}
	return out
}

// encValuePrefix 与客户端 secrets 包的密文前缀逐字一致（enc:v1:）。带该前缀的变量值
// 是客户端口令加密的密文：服务端只归档存储（is_secret 标注加密），**不解析、不解密、
// 不参与渲染/Mock/代码生成**——本产品阶段的既定策略，占位符按「未定义」原样保留。
const encValuePrefix = "enc:v1:"

func isEncryptedValue(v string) bool { return strings.HasPrefix(v, encValuePrefix) }
