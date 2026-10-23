package service

// 同步载荷构造（P2-14：从 sync.go 拆出）。
// 增量（loadPayload）与全量快照（snapshotRows）必须同口径，改这里要两边一起看。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"apidoc/internal/model"
)

// loadPayload 把实体序列化为同步载荷。
// reveal 表示调用方具备明文权限（审核 H-11）：明文 secret 只对这类调用方下发。
func (s *Service) loadPayload(ctx context.Context, projectID uint64, entityType, extUID string, reveal bool) (json.RawMessage, error) {
	switch entityType {
	case model.SyncTypeGroup:
		g, err := s.repo.FindGroupByExtUID(ctx, projectID, extUID)
		if err != nil || g == nil {
			return nil, errors.New("not found")
		}
		p := SyncPayload{Title: g.Title, Priority: g.Priority}
		if g.PID > 0 {
			p.PID = s.repo.GroupExtUID(ctx, g.PID)
		}
		return json.Marshal(p)
	case model.SyncTypeAPI:
		a, err := s.repo.FindApiByExtUID(ctx, projectID, extUID)
		if err != nil || a == nil {
			return nil, errors.New("not found")
		}
		return json.Marshal(s.apiSyncPayload(ctx, a))
	case model.SyncTypeDoc:
		return json.Marshal(s.docSyncPayload(ctx, projectID, extUID))
	case model.SyncTypeExample:
		ex, err := s.repo.FindApiReturnSampleByExtUID(ctx, projectID, extUID)
		if err != nil || ex == nil {
			return nil, errors.New("not found")
		}
		return json.Marshal(s.exampleSyncPayload(ctx, ex))
	case model.SyncTypeVarEnv:
		e, err := s.repo.FindVarEnvByExtUID(ctx, projectID, extUID)
		if err != nil || e == nil {
			return nil, errors.New("not found")
		}
		return json.Marshal(SyncPayload{Name: e.Name})
	case model.SyncTypeVariable:
		v, err := s.repo.FindVariableByExtUID(ctx, projectID, extUID)
		if err != nil || v == nil {
			return nil, errors.New("not found")
		}
		return json.Marshal(s.varSyncPayload(ctx, v, reveal))
	case model.SyncTypeFieldMapping:
		fm, err := s.repo.FindFieldMappingByExtUID(ctx, projectID, extUID)
		if err != nil || fm == nil {
			return nil, errors.New("not found")
		}
		return json.Marshal(fieldMappingSyncPayload(fm))
	}
	return nil, fmt.Errorf("unknown type %s", entityType)
}

// exampleSyncPayload 构造响应示例的同步载荷。
func (s *Service) exampleSyncPayload(ctx context.Context, ex *model.ApiReturnSample) SyncPayload {
	p := SyncPayload{
		Name: ex.Name, Sample: ex.SampleText, Status: ex.Status,
		ContentType: ex.ContentType, Seq: ex.Seq,
	}
	if ex.ApiID > 0 {
		p.APIUID = s.repo.ApiExtUID(ctx, ex.ApiID)
	}
	return p
}

// varSyncPayload 构造变量同步载荷：env_id 反查 env_uid；值按调用方权限回传（H-11）。
func (s *Service) varSyncPayload(ctx context.Context, v *model.Variable, reveal bool) SyncPayload {
	enabled := v.Enabled == model.VarEnabled
	p := SyncPayload{
		Name: v.Name, Secret: v.IsSecret == model.VarSecret, Enabled: &enabled,
	}
	p.Value, p.Redacted = syncVariableValue(v, reveal)
	if v.EnvID > 0 {
		p.EnvUID = s.repo.VarEnvExtUID(ctx, v.EnvID)
	}
	return p
}

// syncVariableValue 决定 secret 变量在同步载荷里的值（审核 H-11）：
//   - 非 secret / 空值：原样回传；
//   - 客户端密文（enc:v1:）：原样回传——服务端不解析，客户端可用口令解密后使用；
//   - 明文 secret：有明文权限（管理员/负责人/update 位）时原样回传并记审计，否则置 Redacted 不下发。
func syncVariableValue(v *model.Variable, reveal bool) (value string, redacted bool) {
	if v.IsSecret != model.VarSecret || v.Value == "" {
		return v.Value, false
	}
	if isEncryptedValue(v.Value) || reveal {
		return v.Value, false
	}
	return "", true
}

// exampleSyncPayloadWith 快照路径用：所属接口 uid 从已建好的 map 取，避免逐行查询。
func (s *Service) exampleSyncPayloadWith(ex *model.ApiReturnSample, apiUID map[uint64]string) SyncPayload {
	p := SyncPayload{
		Name: ex.Name, Sample: ex.SampleText, Status: ex.Status,
		ContentType: ex.ContentType, Seq: ex.Seq,
	}
	p.APIUID = apiUID[ex.ApiID]
	return p
}

// varSyncPayloadWith 快照路径用：所属环境 uid 从已建好的 map 取。
func (s *Service) varSyncPayloadWith(v *model.Variable, envUID map[uint64]string, reveal bool) SyncPayload {
	enabled := v.Enabled == model.VarEnabled
	p := SyncPayload{
		Name: v.Name, Secret: v.IsSecret == model.VarSecret, Enabled: &enabled,
	}
	p.Value, p.Redacted = syncVariableValue(v, reveal)
	p.EnvUID = envUID[v.EnvID]
	return p
}

// maxAuditSecretNames 审计文案里最多列出的变量名数（避免一次快照写出超长日志）。
const maxAuditSecretNames = 20

// auditSyncSecretReveal 在同步下发明文敏感值时写一条审计（审核 H-11）：
// 与 /api/variable?with_value=1 同口径；密文与已脱敏条目不算。
func (s *Service) auditSyncSecretReveal(ctx context.Context, userID, projectID uint64, items []SyncItem) {
	names := make([]string, 0, 4)
	for _, it := range items {
		if it.Type != model.SyncTypeVariable || len(it.Payload) == 0 {
			continue
		}
		var p SyncPayload
		if json.Unmarshal(it.Payload, &p) != nil || !p.Secret || p.Redacted || p.Value == "" || isEncryptedValue(p.Value) {
			continue
		}
		names = append(names, p.Name)
	}
	if len(names) == 0 {
		return
	}
	if len(names) > maxAuditSecretNames {
		names = append(names[:maxAuditSecretNames], "…")
	}
	s.recordOp(ctx, projectID, userID, 0, model.LogObjectVariable, "query", "同步读取敏感变量明文："+strings.Join(names, ", "))
}

// fieldMappingSyncPayload 构造字段字典同步载荷（字段路径 / 类型 / 说明）。
func fieldMappingSyncPayload(fm *model.FieldMapping) SyncPayload {
	return SyncPayload{Field: fm.Field, FieldType: fm.Type, Description: fm.Description}
}

// applyApiExtra 把同步 extra（JSON 文本）逐键写入 api 文本列：键存在且为字符串才覆盖
// （空串=清空该列），缺键不动 —— 与 apiSyncPayload 的键集严格对齐，保证旧客户端推旧键集
// 不会清空服务端新列，反之亦然。富字段均以原始 JSON/文本存列，服务端不解释语义。
func applyApiExtra(e *model.Api, raw string) {
	if raw == "" {
		return
	}
	var extra map[string]any
	if json.Unmarshal([]byte(raw), &extra) != nil {
		return
	}
	setStr := func(key string, dst *string) {
		if v, ok := extra[key].(string); ok {
			*dst = v
		}
	}
	setStr("headers", &e.HttpRequestHeader)
	setStr("params", &e.HttpRequestParams)
	setStr("retType", &e.HttpReturnType)
	setStr("retParams", &e.HttpReturnParams)
	setStr("retSample", &e.HttpReturnSample)
	setStr("desc", &e.Description)
	setStr("protocol", &e.ProtocolType)
	// 客户端富字段（S16 扩展）
	setStr("bodyType", &e.HttpRequestBodyType)
	setStr("body", &e.HttpRequestBody)
	setStr("bodyForm", &e.HttpRequestBodyForm)
	setStr("bodyDescs", &e.HttpRequestBodyDescs)
	setStr("auth", &e.HttpAuth)
	setStr("settings", &e.RequestSettings)
	setStr("script", &e.HttpScript)
	setStr("vars", &e.ScriptVars)
	setStr("asserts", &e.ScriptAsserts)
	setStr("retContentType", &e.HttpReturnContentType)
}

// apiSyncPayload 构造 api 条目的同步载荷：名称/方法/URL/分组 + extra 全量原文。
// 增量（loadPayload）与全量快照（SyncSnapshot）必须同口径，避免两条拉取路径数据不一致。
// docSyncPayload 文档的同步载荷（J11）：group_id → group_uid 回传，
// 客户端据此把文档落到对应目录；找不到记录返回空载荷。
func (s *Service) docSyncPayload(ctx context.Context, projectID uint64, extUID string) SyncPayload {
	d, err := s.repo.FindDocByExtUID(ctx, projectID, extUID)
	if err != nil || d == nil {
		return SyncPayload{}
	}
	p := SyncPayload{Title: d.Title, Content: d.Content, Icon: d.Icon}
	if d.GroupID > 0 {
		p.GroupUID = s.repo.GroupExtUID(ctx, d.GroupID)
	}
	return p
}

func (s *Service) apiSyncPayload(ctx context.Context, a *model.Api) SyncPayload {
	return s.apiSyncPayloadWith(ctx, a, nil)
}

// apiSyncPayloadWith 同上，但所属分组 uid 可由调用方预先建好的 map 提供（map 为 nil 时回退单查）。
func (s *Service) apiSyncPayloadWith(ctx context.Context, a *model.Api, groupUID map[uint64]string) SyncPayload {
	p := SyncPayload{Name: a.ApiName, Method: a.HttpMethodType, URL: a.URL}
	if a.GroupID > 0 {
		if groupUID != nil {
			p.GroupUID = groupUID[a.GroupID]
		} else {
			p.GroupUID = s.repo.GroupExtUID(ctx, a.GroupID)
		}
	}
	// Extra：把接口全量原文聚合（S11）。值均为 DB 中的 JSON 文本列（空串=未设置），
	// 客户端按 type 自行解析；新增键时 push 侧（applyUpsert）也要同步接收。
	extra := map[string]any{
		"headers":   a.HttpRequestHeader,
		"params":    a.HttpRequestParams,
		"retType":   a.HttpReturnType,
		"retParams": a.HttpReturnParams,
		"retSample": a.HttpReturnSample,
		"desc":      a.Description,
		"protocol":  a.ProtocolType,
		// 客户端富字段（S16 扩展）回传，键集与 applyApiExtra 严格对齐；空串=该列未设置。
		"bodyType":       a.HttpRequestBodyType,
		"body":           a.HttpRequestBody,
		"bodyForm":       a.HttpRequestBodyForm,
		"bodyDescs":      a.HttpRequestBodyDescs,
		"auth":           a.HttpAuth,
		"settings":       a.RequestSettings,
		"script":         a.HttpScript,
		"vars":           a.ScriptVars,
		"asserts":        a.ScriptAsserts,
		"retContentType": a.HttpReturnContentType,
	}
	eb, _ := json.Marshal(extra)
	p.Extra = string(eb)
	return p
}
