package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"apidoc/internal/model"
)

// SyncProto 当前同步协议版本（能力协商）。
const SyncProto = 1

// ServerVersion 服务端版本（能力协商）。
const ServerVersion = "1.0.0"

// ---- DTO ----

// SyncStateResp GET /api/sync/state
type SyncStateResp struct {
	Cursor          int64 `json:"cursor"`
	OldestAvailable int64 `json:"oldest_available"`
}

// SyncItem 增量/快照里的一条。
type SyncItem struct {
	Type    string          `json:"type"`
	UID     string          `json:"uid"`
	Op      string          `json:"op"`
	Payload json.RawMessage `json:"payload,omitempty"`
	ItemRev int64           `json:"item_rev"`
	Deleted bool            `json:"deleted,omitempty"`
}

// SyncChangesResp GET /api/sync/changes
type SyncChangesResp struct {
	Cursor  int64      `json:"cursor"`
	Items   []SyncItem `json:"items"`
	HasMore bool       `json:"has_more"`
	Gap     bool       `json:"gap"`
}

// SyncSnapshotResp GET /api/sync/snapshot
type SyncSnapshotResp struct {
	Cursor  int64      `json:"cursor"`
	Items   []SyncItem `json:"items"`
	HasMore bool       `json:"has_more"`
	Offset  int        `json:"offset"`
}

// SyncOpIn push 的一条。
type SyncOpIn struct {
	OpID    string          `json:"op_id"`
	Op      string          `json:"op"`
	Type    string          `json:"type"`
	UID     string          `json:"uid"`
	BaseRev int64           `json:"base_rev"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// SyncPushReq POST /api/sync/push
type SyncPushReq struct {
	ProjectID  uint64     `json:"project_id"`
	DeviceID   string     `json:"device_id"`
	DeviceName string     `json:"device_name"`
	Ops        []SyncOpIn `json:"ops"`
}

// SyncOpOut 逐条结果。
type SyncOpOut struct {
	OpID    string          `json:"op_id"`
	Status  string          `json:"status"` // ok | conflict | rejected
	ItemRev int64           `json:"item_rev,omitempty"`
	Reason  string          `json:"reason,omitempty"`
	Server  json.RawMessage `json:"server,omitempty"` // conflict 时的服务端版本
}

// SyncPushResp POST /api/sync/push
type SyncPushResp struct {
	Results []SyncOpOut `json:"results"`
	Cursor  int64       `json:"cursor"`
}

// MetaResp GET /api/meta 能力协商。
type MetaResp struct {
	ServerVersion    string   `json:"server_version"`
	APIVersion       string   `json:"api_version"`
	SyncProto        int      `json:"sync_proto"`
	Features         []string `json:"features"`
	MinClientVersion string   `json:"min_client_version"`
}

// SyncPayload 条目序列化载荷（客户端按 type 解析）。
type SyncPayload struct {
	// group
	Title    string `json:"title,omitempty"`
	PID      string `json:"parent_uid,omitempty"`
	Priority int    `json:"priority,omitempty"`
	// api
	Name     string `json:"name,omitempty"`
	Method   string `json:"method,omitempty"`
	URL      string `json:"url,omitempty"`
	GroupUID string `json:"group_uid,omitempty"`
	Extra    string `json:"extra,omitempty"` // 客户端全量原文 JSON（S11）
	// doc
	Content string `json:"content,omitempty"`
	Icon    string `json:"icon,omitempty"`
	// example（响应示例）：Name 复用为示例名
	ApiUID      string `json:"api_uid,omitempty"`
	Sample      string `json:"sample,omitempty"`
	Status      int    `json:"status,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Seq         int    `json:"seq,omitempty"`
	// variable / variable_env（S16）：Name 复用为变量名/环境名；secret 变量的 Value
	// 为客户端密文（enc:v1: 前缀）——服务端只存储、不解析，值本身即「加密数据」标注。
	EnvUID  string `json:"env_uid,omitempty"` // 变量所属环境的 ext_uid
	Value   string `json:"value,omitempty"`   // 变量值（明文或 enc:v1: 密文）
	Secret  bool   `json:"secret,omitempty"`  // 是否敏感值
	Enabled *bool  `json:"enabled,omitempty"` // 是否启用（nil 视作启用）
	// 通用
	Deleted bool `json:"deleted,omitempty"`
}

// ---- Service ----

// Meta 能力协商。
func (s *Service) Meta(ctx context.Context) *MetaResp {
	return &MetaResp{
		ServerVersion:    ServerVersion,
		APIVersion:       "1",
		SyncProto:        SyncProto,
		Features:         []string{"sync", "pat", "change_log", "tombstone", "op_id"},
		MinClientVersion: "0.1.0",
	}
}

// SyncState 探测游标。
func (s *Service) SyncState(ctx context.Context, projectID uint64) (*SyncStateResp, error) {
	cur, err := s.repo.LatestChangeID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	old, err := s.repo.OldestChangeID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return &SyncStateResp{Cursor: cur, OldestAvailable: old}, nil
}

// SyncChanges 增量拉取。
func (s *Service) SyncChanges(ctx context.Context, projectID uint64, since int64, limit int) (*SyncChangesResp, error) {
	if limit <= 0 {
		limit = 200
	}
	old, err := s.repo.OldestChangeID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	// gap：since 早于流水保留窗口
	if old > 0 && since > 0 && since < old-1 {
		return &SyncChangesResp{Gap: true, Cursor: since}, nil
	}

	logs, err := s.repo.ChangesSince(ctx, projectID, since, limit+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(logs) > limit
	if hasMore {
		logs = logs[:limit]
	}
	items := make([]SyncItem, 0, len(logs))
	for _, lg := range logs {
		it := SyncItem{Type: lg.EntityType, UID: lg.ExtUID, Op: lg.Op, ItemRev: lg.ID}
		if lg.Op == model.SyncOpDelete {
			it.Deleted = true
		} else {
			payload, err := s.loadPayload(ctx, projectID, lg.EntityType, lg.ExtUID)
			if err == nil {
				it.Payload = payload
			}
		}
		items = append(items, it)
	}
	cursor := since
	if len(logs) > 0 {
		cursor = logs[len(logs)-1].ID
	}
	cur, _ := s.repo.LatestChangeID(ctx, projectID)
	if !hasMore && cursor < cur {
		cursor = cur
	}
	return &SyncChangesResp{Cursor: cursor, Items: items, HasMore: hasMore}, nil
}

// SyncSnapshot 全量分页。
func (s *Service) SyncSnapshot(ctx context.Context, projectID uint64, offset, limit int) (*SyncSnapshotResp, error) {
	if limit <= 0 {
		limit = 200
	}
	type row struct {
		typ     string
		uid     string
		deleted bool
		rev     int64
		payload json.RawMessage
	}
	var rows []row

	groups, err := s.repo.ListGroupsAll(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		if g.ExtUID == "" {
			continue
		}
		p := SyncPayload{Title: g.Title, Priority: g.Priority, Deleted: g.IsDeleted == model.IsDeletedYes}
		if g.PID > 0 {
			if parent, _ := s.repo.FindGroupByExtUID(ctx, projectID, ""); parent != nil {
				_ = parent
			}
			// parent_uid 通过 p_id 反查
			var pg model.Group
			if err := s.repo.DB().WithContext(ctx).Where("id = ?", g.PID).First(&pg).Error; err == nil {
				p.PID = pg.ExtUID
			}
		}
		b, _ := json.Marshal(p)
		rows = append(rows, row{model.SyncTypeGroup, g.ExtUID, p.Deleted, g.LastChangeID, b})
	}
	apis, err := s.repo.ListApisAll(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, a := range apis {
		if a.ExtUID == "" {
			continue
		}
		// 与增量路径（loadPayload）同口径；快照含软删行，需要补 Deleted 标记。
		p := s.apiSyncPayload(ctx, &a)
		p.Deleted = a.IsDeleted == model.IsDeletedYes
		b, _ := json.Marshal(p)
		rows = append(rows, row{model.SyncTypeAPI, a.ExtUID, p.Deleted, a.LastChangeID, b})
	}
	docs, err := s.repo.ListDocsAll(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, d := range docs {
		if d.ExtUID == "" {
			continue
		}
		p := SyncPayload{Title: d.Title, Content: d.Content, Icon: d.Icon, Deleted: d.IsDeleted == model.IsDeletedYes}
		if d.GroupID > 0 {
			var g model.Group
			if err := s.repo.DB().WithContext(ctx).Where("id = ?", d.GroupID).First(&g).Error; err == nil {
				p.GroupUID = g.ExtUID
			}
		}
		b, _ := json.Marshal(p)
		rows = append(rows, row{model.SyncTypeDoc, d.ExtUID, p.Deleted, d.LastChangeID, b})
	}
	examples, err := s.repo.ListApiReturnSamplesAll(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, ex := range examples {
		if ex.ExtUID == "" {
			continue
		}
		p := s.exampleSyncPayload(ctx, &ex)
		p.Deleted = ex.IsDeleted == model.IsDeletedYes
		b, _ := json.Marshal(p)
		rows = append(rows, row{model.SyncTypeExample, ex.ExtUID, p.Deleted, ex.LastChangeID, b})
	}
	// 变量类：环境先于变量（拓扑序，变量按 env_uid 挂接）。
	varEnvs, err := s.repo.ListVarEnvsAll(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, ev := range varEnvs {
		if ev.ExtUID == "" {
			continue
		}
		p := SyncPayload{Name: ev.Name, Deleted: ev.IsDeleted == model.IsDeletedYes}
		b, _ := json.Marshal(p)
		rows = append(rows, row{model.SyncTypeVarEnv, ev.ExtUID, p.Deleted, ev.LastChangeID, b})
	}
	vars, err := s.repo.ListVariablesAll(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, v := range vars {
		if v.ExtUID == "" {
			continue
		}
		p := s.varSyncPayload(ctx, &v)
		p.Deleted = v.IsDeleted == model.IsDeletedYes
		b, _ := json.Marshal(p)
		rows = append(rows, row{model.SyncTypeVariable, v.ExtUID, p.Deleted, v.LastChangeID, b})
	}

	if offset < 0 {
		offset = 0
	}
	if offset >= len(rows) {
		cur, _ := s.repo.LatestChangeID(ctx, projectID)
		return &SyncSnapshotResp{Cursor: cur, Offset: offset}, nil
	}
	end := offset + limit
	hasMore := end < len(rows)
	if hasMore {
		rows = rows[offset:end]
	} else {
		rows = rows[offset:]
	}
	items := make([]SyncItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, SyncItem{
			Type: r.typ, UID: r.uid, Op: model.SyncOpUpsert, Payload: r.payload,
			ItemRev: r.rev, Deleted: r.deleted,
		})
	}
	cur, _ := s.repo.LatestChangeID(ctx, projectID)
	return &SyncSnapshotResp{Cursor: cur, Items: items, HasMore: hasMore, Offset: offset}, nil
}

// loadPayload 把实体序列化为同步载荷。
func (s *Service) loadPayload(ctx context.Context, projectID uint64, entityType, extUID string) (json.RawMessage, error) {
	switch entityType {
	case model.SyncTypeGroup:
		g, err := s.repo.FindGroupByExtUID(ctx, projectID, extUID)
		if err != nil || g == nil {
			return nil, errors.New("not found")
		}
		p := SyncPayload{Title: g.Title, Priority: g.Priority}
		if g.PID > 0 {
			var pg model.Group
			if err := s.repo.DB().WithContext(ctx).Where("id = ?", g.PID).First(&pg).Error; err == nil {
				p.PID = pg.ExtUID
			}
		}
		return json.Marshal(p)
	case model.SyncTypeAPI:
		a, err := s.repo.FindApiByExtUID(ctx, projectID, extUID)
		if err != nil || a == nil {
			return nil, errors.New("not found")
		}
		return json.Marshal(s.apiSyncPayload(ctx, a))
	case model.SyncTypeDoc:
		d, err := s.repo.FindDocByExtUID(ctx, projectID, extUID)
		if err != nil || d == nil {
			return nil, errors.New("not found")
		}
		p := SyncPayload{Title: d.Title, Content: d.Content, Icon: d.Icon}
		if d.GroupID > 0 {
			var g model.Group
			if err := s.repo.DB().WithContext(ctx).Where("id = ?", d.GroupID).First(&g).Error; err == nil {
				p.GroupUID = g.ExtUID
			}
		}
		return json.Marshal(p)
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
		return json.Marshal(s.varSyncPayload(ctx, v))
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
		var a model.Api
		if err := s.repo.DB().WithContext(ctx).Where("id = ?", ex.ApiID).First(&a).Error; err == nil {
			p.ApiUID = a.ExtUID
		}
	}
	return p
}

// varSyncPayload 构造变量同步载荷：env_id 反查 env_uid；值原样回传（enc:v1: 密文不解析）。
func (s *Service) varSyncPayload(ctx context.Context, v *model.Variable) SyncPayload {
	enabled := v.Enabled == model.VarEnabled
	p := SyncPayload{
		Name: v.Name, Value: v.Value,
		Secret: v.IsSecret == model.VarSecret, Enabled: &enabled,
	}
	if v.EnvID > 0 {
		var env model.VariableEnv
		if err := s.repo.DB().WithContext(ctx).Where("id = ?", v.EnvID).First(&env).Error; err == nil {
			p.EnvUID = env.ExtUID
		}
	}
	return p
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
func (s *Service) apiSyncPayload(ctx context.Context, a *model.Api) SyncPayload {
	p := SyncPayload{Name: a.ApiName, Method: a.HttpMethodType, URL: a.URL}
	if a.GroupID > 0 {
		var g model.Group
		if err := s.repo.DB().WithContext(ctx).Where("id = ?", a.GroupID).First(&g).Error; err == nil {
			p.GroupUID = g.ExtUID
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

// SyncPush 批量提交（幂等 + 拓扑序 + 冲突检测）。typ 为主体用户类型，管理员放行、其余需项目成员 + 修改权限位。
func (s *Service) SyncPush(ctx context.Context, userID uint64, typ int8, req *SyncPushReq) (*SyncPushResp, error) {
	if req == nil || req.ProjectID == 0 {
		return nil, errors.New("project_id 必填")
	}
	// 写开关（S14）
	proj, err := s.repo.GetProjectAny(ctx, req.ProjectID)
	if err != nil {
		return nil, err
	}
	// 权限位（§4.2）：push 与 Web 写路径同口径，须为项目成员且具备修改权限（管理员放行）。
	// 堵住此前「任意持有效令牌者可向任意 project_id 推送」的欠账。
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	writeDisabled := proj != nil && proj.ClientWriteEnabled != nil && !*proj.ClientWriteEnabled

	out := make([]SyncOpOut, 0, len(req.Ops))
	for _, op := range req.Ops {
		// 幂等
		if prev, err := s.repo.GetSyncOp(ctx, op.OpID); err == nil && prev != nil {
			out = append(out, SyncOpOut{
				OpID: op.OpID, Status: prev.Status, ItemRev: prev.ItemRev, Reason: prev.Reason,
			})
			continue
		}
		res := s.applyOp(ctx, userID, req, op, writeDisabled)
		_ = s.repo.SaveSyncOp(ctx, &model.SyncOp{
			OpID: op.OpID, ProjectID: req.ProjectID,
			Status: res.Status, ItemRev: res.ItemRev, Reason: res.Reason,
		})
		out = append(out, res)
	}
	cur, _ := s.repo.LatestChangeID(ctx, req.ProjectID)
	return &SyncPushResp{Results: out, Cursor: cur}, nil
}

// applyOp 处理单条 push。
func (s *Service) applyOp(ctx context.Context, userID uint64, req *SyncPushReq, op SyncOpIn, writeDisabled bool) SyncOpOut {
	if writeDisabled {
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "readonly"}
	}
	switch op.Type {
	case model.SyncTypeGroup, model.SyncTypeAPI, model.SyncTypeDoc, model.SyncTypeExample,
		model.SyncTypeVarEnv, model.SyncTypeVariable:
	default:
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
	}
	if op.UID == "" {
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
	}

	// 冲突检测：base_rev != last_change_id ⇒ conflict（新建 base_rev=0 允许）
	var curRev int64
	var exists bool
	switch op.Type {
	case model.SyncTypeGroup:
		e, _ := s.repo.FindGroupByExtUID(ctx, req.ProjectID, op.UID)
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeAPI:
		e, _ := s.repo.FindApiByExtUID(ctx, req.ProjectID, op.UID)
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeDoc:
		e, _ := s.repo.FindDocByExtUID(ctx, req.ProjectID, op.UID)
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeExample:
		e, _ := s.repo.FindApiReturnSampleByExtUID(ctx, req.ProjectID, op.UID)
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeVarEnv:
		e, _ := s.repo.FindVarEnvByExtUID(ctx, req.ProjectID, op.UID)
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeVariable:
		e, _ := s.repo.FindVariableByExtUID(ctx, req.ProjectID, op.UID)
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	}
	if exists && op.BaseRev > 0 && op.BaseRev != curRev {
		payload, _ := s.loadPayload(ctx, req.ProjectID, op.Type, op.UID)
		return SyncOpOut{
			OpID: op.OpID, Status: "conflict", ItemRev: curRev, Server: payload,
		}
	}

	if op.Op == model.SyncOpDelete {
		return s.applyDelete(ctx, userID, req, op)
	}
	return s.applyUpsert(ctx, userID, req, op)
}

func (s *Service) applyUpsert(ctx context.Context, userID uint64, req *SyncPushReq, op SyncOpIn) SyncOpOut {
	var p SyncPayload
	if len(op.Payload) > 0 {
		_ = json.Unmarshal(op.Payload, &p)
	}
	device := req.DeviceID
	if req.DeviceName != "" {
		device = req.DeviceName
	}

	switch op.Type {
	case model.SyncTypeGroup:
		e, _ := s.repo.FindGroupByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			e = &model.Group{ProjectID: req.ProjectID, Type: model.GroupTypeAPI}
		}
		e.ExtUID = op.UID
		e.Title = p.Title
		e.Priority = p.Priority
		if p.PID != "" {
			if pg, _ := s.repo.FindGroupByExtUID(ctx, req.ProjectID, p.PID); pg != nil {
				e.PID = pg.ID
			} else {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
		} else {
			e.PID = 0
		}
		var err error
		if e.ID == 0 {
			err = s.repo.CreateGroup(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = s.repo.UpdateGroup(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.repo.AppendChange(nil, req.ProjectID, model.SyncTypeGroup, op.UID, model.SyncOpUpsert, userID, device)
		if err == nil {
			_ = s.repo.SetLastChange(nil, model.SyncTypeGroup, e.ID, cid)
			e.LastChangeID = cid
		}
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: e.LastChangeID}

	case model.SyncTypeAPI:
		e, _ := s.repo.FindApiByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			e = &model.Api{ProjectID: req.ProjectID}
		}
		e.ExtUID = op.UID
		e.ApiName = p.Name
		e.HttpMethodType = p.Method
		e.URL = p.URL
		if p.GroupUID != "" {
			if g, _ := s.repo.FindGroupByExtUID(ctx, req.ProjectID, p.GroupUID); g != nil {
				e.GroupID = g.ID
			} else {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
		} else {
			e.GroupID = 0
		}
		applyApiExtra(e, p.Extra)
		var err error
		if e.ID == 0 {
			err = s.repo.CreateApi(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = s.repo.UpdateApi(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.repo.AppendChange(nil, req.ProjectID, model.SyncTypeAPI, op.UID, model.SyncOpUpsert, userID, device)
		if err == nil {
			_ = s.repo.SetLastChange(nil, model.SyncTypeAPI, e.ID, cid)
			e.LastChangeID = cid
		}
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: e.LastChangeID}

	case model.SyncTypeDoc:
		e, _ := s.repo.FindDocByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			e = &model.Doc{ProjectID: req.ProjectID, UserID: userID}
		}
		e.ExtUID = op.UID
		e.Title = p.Title
		e.Content = p.Content
		e.Icon = p.Icon
		if p.GroupUID != "" {
			if g, _ := s.repo.FindGroupByExtUID(ctx, req.ProjectID, p.GroupUID); g != nil {
				e.GroupID = g.ID
			} else {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
		} else {
			e.GroupID = 0
		}
		var err error
		if e.ID == 0 {
			err = s.repo.CreateDoc(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = s.repo.UpdateDoc(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.repo.AppendChange(nil, req.ProjectID, model.SyncTypeDoc, op.UID, model.SyncOpUpsert, userID, device)
		if err == nil {
			_ = s.repo.SetLastChange(nil, model.SyncTypeDoc, e.ID, cid)
			e.LastChangeID = cid
		}
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: e.LastChangeID}

	case model.SyncTypeExample:
		e, _ := s.repo.FindApiReturnSampleByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			e = &model.ApiReturnSample{ProjectID: req.ProjectID}
		}
		e.ExtUID = op.UID
		e.Name = p.Name
		e.SampleText = p.Sample
		e.Status = p.Status
		e.ContentType = p.ContentType
		e.Seq = p.Seq
		if p.ApiUID != "" {
			if a, _ := s.repo.FindApiByExtUID(ctx, req.ProjectID, p.ApiUID); a != nil {
				e.ApiID = a.ID
			} else {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
		}
		var err error
		if e.ID == 0 {
			err = s.repo.CreateApiReturnSample(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = s.repo.UpdateApiReturnSample(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.repo.AppendChange(nil, req.ProjectID, model.SyncTypeExample, op.UID, model.SyncOpUpsert, userID, device)
		if err == nil {
			_ = s.repo.SetLastChange(nil, model.SyncTypeExample, e.ID, cid)
			e.LastChangeID = cid
		}
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: e.LastChangeID}

	case model.SyncTypeVarEnv:
		// 环境：Name 复用载荷字段；仅建/改同步身份与名称，颜色/描述等 Web 侧属性不覆盖。
		e, _ := s.repo.FindVarEnvByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			e = &model.VariableEnv{ProjectID: req.ProjectID}
		}
		e.ExtUID = op.UID
		if p.Name != "" {
			e.Name = p.Name
		}
		var err error
		if e.ID == 0 {
			err = s.repo.CreateVariableEnv(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = s.repo.UpdateVariableEnv(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.repo.AppendChange(nil, req.ProjectID, model.SyncTypeVarEnv, op.UID, model.SyncOpUpsert, userID, device)
		if err == nil {
			_ = s.repo.SetLastChange(nil, model.SyncTypeVarEnv, e.ID, cid)
			e.LastChangeID = cid
		}
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: e.LastChangeID}

	case model.SyncTypeVariable:
		// 变量：项目级、挂到 env_uid 对应环境；Value 原样存（secret 为 enc:v1: 密文，服务端不解析）。
		e, _ := s.repo.FindVariableByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			e = &model.Variable{ProjectID: req.ProjectID, Scope: model.VarScopeProject}
		}
		e.ExtUID = op.UID
		e.Name = p.Name
		e.Value = p.Value
		e.IsSecret = model.VarNotSecret
		if p.Secret {
			e.IsSecret = model.VarSecret
		}
		e.Enabled = model.VarEnabled
		if p.Enabled != nil && !*p.Enabled {
			e.Enabled = model.VarDisabled
		}
		e.EnvID = 0
		if p.EnvUID != "" {
			env, _ := s.repo.FindVarEnvByExtUID(ctx, req.ProjectID, p.EnvUID)
			if env == nil {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
			e.EnvID = env.ID
		}
		var err error
		if e.ID == 0 {
			err = s.repo.CreateVariable(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = s.repo.UpdateVariable(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.repo.AppendChange(nil, req.ProjectID, model.SyncTypeVariable, op.UID, model.SyncOpUpsert, userID, device)
		if err == nil {
			_ = s.repo.SetLastChange(nil, model.SyncTypeVariable, e.ID, cid)
			e.LastChangeID = cid
		}
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: e.LastChangeID}
	}
	return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
}

func (s *Service) applyDelete(ctx context.Context, userID uint64, req *SyncPushReq, op SyncOpIn) SyncOpOut {
	device := req.DeviceID
	if req.DeviceName != "" {
		device = req.DeviceName
	}
	var id uint64
	switch op.Type {
	case model.SyncTypeGroup:
		e, _ := s.repo.FindGroupByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := s.repo.DeleteGroup(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeAPI:
		e, _ := s.repo.FindApiByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := s.repo.DeleteApi(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeDoc:
		e, _ := s.repo.FindDocByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := s.repo.DeleteDoc(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeExample:
		e, _ := s.repo.FindApiReturnSampleByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := s.repo.DeleteApiReturnSample(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeVarEnv:
		e, _ := s.repo.FindVarEnvByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := s.repo.DeleteVariableEnv(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeVariable:
		e, _ := s.repo.FindVariableByExtUID(ctx, req.ProjectID, op.UID)
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := s.repo.DeleteVariable(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	default:
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
	}
	cid, err := s.repo.AppendChange(nil, req.ProjectID, op.Type, op.UID, model.SyncOpDelete, userID, device)
	if err == nil {
		_ = s.repo.SetLastChange(nil, op.Type, id, cid)
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}
	}
	return SyncOpOut{OpID: op.OpID, Status: "ok"}
}
