package service

// 变量解析与汇总（P2-14：从 variable.go 拆出）。
// 规则唯一真源是 share/varx；这里只负责把 DB 里的变量拼成它的输入表。

import (
	"context"
	"sort"

	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"

	"github.com/leihenshang/api-doc-go-share/varx"
)

// ResolveVariables 解析接口可见变量表：同名只保留最高优先级的一条，敏感值掩码。
func (s *Service) ResolveVariables(ctx context.Context, projectID, apiID, envID uint64) ([]ResolveItem, error) {
	if projectID == 0 {
		return nil, newBiz(errcode.CodeParamError, "缺少 project_id")
	}
	vars, err := s.repo.VariablesForResolve(ctx, projectID, apiID)
	if err != nil {
		return nil, err
	}
	best := map[string]resolvedVar{}
	for _, v := range vars {
		rank, ok := varRank(v, envID)
		if !ok {
			continue
		}
		if cur, exists := best[v.Name]; !exists || rank < cur.rank {
			best[v.Name] = resolvedVar{v: v, rank: rank}
		}
	}
	names := make([]string, 0, len(best))
	for name := range best {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ResolveItem, 0, len(names))
	for _, name := range names {
		hit := best[name]
		out = append(out, ResolveItem{VarView: maskVariable(hit.v), Source: varSourceOf(hit.v)})
	}
	return out, nil
}

// SummarizeVariables 批量统计接口的变量使用情况（接口列表徽标）。
// defined = 该接口的请求变量名；undefined = 接口文本中引用但四层都未命中的变量名。
func (s *Service) SummarizeVariables(ctx context.Context, projectID, envID uint64, apiIDs []uint64) (map[uint64]VarSummary, error) {
	out := map[uint64]VarSummary{}
	if projectID == 0 {
		return nil, newBiz(errcode.CodeParamError, "缺少 project_id")
	}
	if len(apiIDs) == 0 {
		return nil, newBiz(errcode.CodeParamError, "缺少 api_ids")
	}
	if len(apiIDs) > maxSummaryApiIDs {
		return nil, newBiz(errcode.CodeParamError, "单次最多查询 %d 个接口，请分批请求", maxSummaryApiIDs)
	}
	apis, err := s.repo.ApisByIDs(ctx, projectID, apiIDs)
	if err != nil {
		return nil, err
	}
	vars, err := s.repo.VariablesForApis(ctx, projectID, apiIDs)
	if err != nil {
		return nil, err
	}
	// 共享层（项目级 + 全局）对所有接口一致；请求级按接口归组。
	shared := map[string]struct{}{}
	byAPI := map[uint64]map[string]struct{}{}
	for _, v := range vars {
		if v.Enabled != model.VarEnabled {
			continue
		}
		if v.Scope == model.VarScopeRequest {
			if byAPI[v.ApiID] == nil {
				byAPI[v.ApiID] = map[string]struct{}{}
			}
			byAPI[v.ApiID][v.Name] = struct{}{}
			continue
		}
		if _, ok := varRank(v, envID); ok {
			shared[v.Name] = struct{}{}
		}
	}
	for _, a := range apis {
		reqVars := byAPI[a.ID]
		defined := make([]string, 0, len(reqVars))
		for name := range reqVars {
			defined = append(defined, name)
		}
		sort.Strings(defined)
		undefined := make([]string, 0)
		for _, name := range apiVarNames(&a) {
			if _, ok := reqVars[name]; ok {
				continue
			}
			if _, ok := shared[name]; ok {
				continue
			}
			undefined = append(undefined, name)
		}
		out[a.ID] = VarSummary{
			Defined:        defined,
			Undefined:      undefined,
			RequestCount:   len(defined),
			UndefinedCount: len(undefined),
		}
	}
	return out, nil
}

// resolveTable 返回该接口在当前环境下同名**最高优先级**的变量（保留原文，含敏感值）。
// 仅供服务端渲染 cURL / 代码片段使用；对外接口仍走 ResolveVariables 的脱敏视图。
// 客户端加密的 secret（enc:v1: 前缀）一律跳过——服务端不解析密文，占位符保持原样。
func (s *Service) resolveTable(ctx context.Context, projectID, apiID, envID uint64) (map[string]model.Variable, error) {
	vars, err := s.repo.VariablesForResolve(ctx, projectID, apiID)
	if err != nil {
		return nil, err
	}
	best := map[string]resolvedVar{}
	for _, v := range vars {
		if isEncryptedValue(v.Value) {
			continue
		}
		rank, ok := varRank(v, envID)
		if !ok {
			continue
		}
		if cur, exists := best[v.Name]; !exists || rank < cur.rank {
			best[v.Name] = resolvedVar{v: v, rank: rank}
		}
	}
	table := make(map[string]model.Variable, len(best))
	for name, hit := range best {
		table[name] = hit.v
	}
	return table, nil
}

// renderVars 把文本中的 {{name}} 替换为变量值：未定义保留占位符原文，空值替换为空串。
// table 为空时原样返回（resolve=false 的"保留占位符"模式，连内置变量也不渲染）。
// 表中保留敏感值原文：渲染结果只在服务端使用（cURL/代码片段），单条变量明文不对外回传。
func renderVars(text string, table map[string]model.Variable) string {
	if text == "" || len(table) == 0 {
		return text
	}
	vars := make([]varx.Var, 0, len(table))
	for _, v := range table {
		// 表内已按优先级去重，统一按请求级参与即可保持唯一胜出者。
		sv := toShareVar(v)
		sv.Scope = varx.ScopeRequest
		vars = append(vars, sv)
	}
	out, _ := varx.Build(vars, 0).Resolve(text)
	return out
}

// referencedSecretVars 返回文本引用中命中敏感变量的变量名（去重保序），供导出审计。
func referencedSecretVars(texts []string, table map[string]model.Variable) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, text := range texts {
		for _, name := range collectVarNames(text) {
			hit, ok := table[name]
			if !ok || hit.IsSecret != model.VarSecret {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}
