package service

// RM-08 契约校验：把 OpenAPI 3.x 与项目当前接口做只读 diff。
// 只比较请求侧（method+path+参数），输出新增/删除/必填/类型变化与破坏性标记；
// 响应字段级校验依赖 Schema 模型，暂不在此实现。

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"

	shopenapi "github.com/leihenshang/api-doc-go-share/openapi"
)

const (
	contractKindParamAdded     = "param_added"
	contractKindParamRemoved   = "param_removed"
	contractKindRequiredChange = "required_changed"
	contractKindTypeChange     = "type_changed"
)

// contractParam 当前接口参数归一后的形状。
type contractParam struct {
	Name     string
	In       string
	Type     string
	Required bool
}

// ContractDiff 比较 OpenAPI 文档与项目现有接口（只读，需项目读权限）。
func (s *Service) ContractDiff(ctx context.Context, userID uint64, typ int8, req dto.ContractDiffReq) (*dto.ContractDiffResult, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermQuery); err != nil {
		return nil, err
	}
	doc, err := shopenapi.Decode(req.OpenAPI)
	if err != nil {
		return nil, newBiz(errcode.CodeParamError, "OpenAPI 解析失败：%v", err)
	}
	apis, err := s.repo.ListApisForExport(ctx, req.ProjectID, 0)
	if err != nil {
		return nil, err
	}

	current := make(map[string]*model.Api, len(apis))
	for i := range apis {
		current[contractKey(apis[i].HttpMethodType, apis[i].URL)] = &apis[i]
	}
	spec := make(map[string]shopenapi.Operation, len(doc.Ops))
	for _, op := range doc.Ops {
		spec[contractKey(op.Method, op.Path)] = op
	}

	out := &dto.ContractDiffResult{
		Added:   make([]dto.ContractEndpoint, 0),
		Removed: make([]dto.ContractEndpoint, 0),
		Changes: make([]dto.ContractChange, 0),
	}
	for key, op := range spec {
		if _, ok := current[key]; !ok {
			out.Added = append(out.Added, dto.ContractEndpoint{Method: upperMethod(op.Method), Path: op.Path})
		}
	}
	for key, a := range current {
		if _, ok := spec[key]; !ok {
			out.Removed = append(out.Removed, dto.ContractEndpoint{Method: upperMethod(a.HttpMethodType), Path: a.URL})
			out.BreakingCount++
		}
	}
	for key, op := range spec {
		a, ok := current[key]
		if !ok {
			continue
		}
		changes := compareContractParams(op, a)
		for i := range changes {
			if changes[i].Breaking {
				out.BreakingCount++
			}
		}
		out.Changes = append(out.Changes, changes...)
	}
	sortContractResult(out)
	return out, nil
}

// compareContractParams 对比一个端点的参数；current 中未被 spec 覆盖的参数视为删除。
func compareContractParams(op shopenapi.Operation, a *model.Api) []dto.ContractChange {
	current := currentContractParams(a)
	out := make([]dto.ContractChange, 0)
	method := upperMethod(a.HttpMethodType)
	path := a.URL
	for _, p := range op.Params {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		cp, ok := current[name]
		if !ok {
			change := dto.ContractChange{
				Method: method, Path: path, Param: name, In: p.In,
				Kind: contractKindParamAdded, After: describeContractParam(p.In, p.Type, p.Required),
				Breaking: p.Required,
			}
			out = append(out, change)
			continue
		}
		delete(current, name)
		if p.Required != cp.Required {
			out = append(out, dto.ContractChange{
				Method: method, Path: path, Param: name, In: p.In,
				Kind:   contractKindRequiredChange,
				Before: describeContractParam(cp.In, cp.Type, cp.Required),
				After:  describeContractParam(p.In, p.Type, p.Required),
				// 由可选变必填是破坏性；放宽必填不拦截。
				Breaking: p.Required && !cp.Required,
			})
		}
		if cp.Type != "" && p.Type != "" && contractType(cp.Type) != contractType(p.Type) {
			out = append(out, dto.ContractChange{
				Method: method, Path: path, Param: name, In: p.In,
				Kind:     contractKindTypeChange,
				Before:   describeContractParam(cp.In, cp.Type, cp.Required),
				After:    describeContractParam(p.In, p.Type, p.Required),
				Breaking: true,
			})
		}
	}
	for name, cp := range current {
		out = append(out, dto.ContractChange{
			Method: method, Path: path, Param: name, In: cp.In,
			Kind: contractKindParamRemoved, Before: describeContractParam(cp.In, cp.Type, cp.Required),
			Breaking: true,
		})
	}
	return out
}

// currentContractParams 把接口的请求参数与请求头归一为 name -> 参数。
func currentContractParams(a *model.Api) map[string]contractParam {
	out := map[string]contractParam{}
	var params []struct {
		FieldName string `json:"fieldName"`
		Type      string `json:"type"`
		Required  bool   `json:"required"`
	}
	if json.Unmarshal([]byte(a.HttpRequestParams), &params) == nil {
		for _, p := range params {
			if name := strings.TrimSpace(p.FieldName); name != "" {
				out[name] = contractParam{Name: name, In: "query", Type: p.Type, Required: p.Required}
			}
		}
	}
	var headers []struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
	}
	if json.Unmarshal([]byte(a.HttpRequestHeader), &headers) == nil {
		for _, h := range headers {
			if name := strings.TrimSpace(h.Name); name != "" {
				if _, exists := out[name]; !exists {
					out[name] = contractParam{Name: name, In: "header", Required: h.Required}
				}
			}
		}
	}
	return out
}

// contractType 把两端的常见类型写法归一，避免 int/integer、bool/boolean 误报。
func contractType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "int", "integer", "int32", "int64":
		return "integer"
	case "bool", "boolean":
		return "boolean"
	case "float", "double", "number":
		return "number"
	case "str", "string":
		return "string"
	case "array", "object":
		return strings.ToLower(strings.TrimSpace(t))
	default:
		return strings.ToLower(strings.TrimSpace(t))
	}
}

// describeContractParam 生成给前端阅读的「in type required」描述。
func describeContractParam(in, typ string, required bool) string {
	parts := make([]string, 0, 3)
	if in != "" {
		parts = append(parts, in)
	}
	if typ != "" {
		parts = append(parts, typ)
	}
	if required {
		parts = append(parts, "required")
	}
	return strings.Join(parts, " ")
}

// contractKey 端点匹配键：方法大写 + 原始路径。
func contractKey(method, path string) string {
	m := strings.ToUpper(strings.TrimSpace(method))
	if m == "" {
		m = "GET"
	}
	return m + " " + strings.TrimSpace(path)
}

// sortContractResult 保证报告顺序稳定（map 遍历无序）。
func sortContractResult(out *dto.ContractDiffResult) {
	sort.Slice(out.Added, func(i, j int) bool {
		return out.Added[i].Path+" "+out.Added[i].Method < out.Added[j].Path+" "+out.Added[j].Method
	})
	sort.Slice(out.Removed, func(i, j int) bool {
		return out.Removed[i].Path+" "+out.Removed[i].Method < out.Removed[j].Path+" "+out.Removed[j].Method
	})
	sort.Slice(out.Changes, func(i, j int) bool {
		a, b := out.Changes[i], out.Changes[j]
		return a.Path+" "+a.Method+" "+a.Param+" "+a.Kind < b.Path+" "+b.Method+" "+b.Param+" "+b.Kind
	})
}
