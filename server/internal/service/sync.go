package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"apidoc/internal/model"
	"apidoc/internal/repository"

	syncapi "github.com/leihenshang/api-doc-go-share/syncapi"
)

// SyncProto 当前同步协议版本（能力协商）；真源在 syncapi。
const SyncProto = syncapi.Proto

// ServerVersion 服务端版本（能力协商）。
const ServerVersion = "1.0.0"

// ---- DTO ----
//
// 同步线协议的唯一真源在共享库 `syncapi`（P2-1）：结构与 `json` tag 都在那边定义，
// 这里只保留服务端惯用的类型别名。这样两端字段不一致在编译期就不可能发生，
// 改协议必须先改共享库（并同步升 `syncapi.Proto` 与两端 require）。

type (
	SyncStateResp    = syncapi.State
	SyncItem         = syncapi.Item
	SyncChangesResp  = syncapi.Changes
	SyncSnapshotResp = syncapi.Snapshot
	SyncOpIn         = syncapi.Op
	SyncPushReq      = syncapi.PushReq
	SyncOpOut        = syncapi.OpResult
	SyncPushResp     = syncapi.PushResp
	MetaResp         = syncapi.Meta
	SyncPayload      = syncapi.Payload
)

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
func (s *Service) SyncState(ctx context.Context, userID uint64, typ int8, projectID uint64) (*SyncStateResp, error) {
	// 越权读防护：未校验时任意登录用户可遍历 project_id 拉取他人项目游标
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermQuery); err != nil {
		return nil, err
	}
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
func (s *Service) SyncChanges(ctx context.Context, userID uint64, typ int8, projectID uint64, since int64, limit int) (*SyncChangesResp, error) {
	// 越权读防护：变更流含全量接口定义（URL/请求头/请求体/响应样例）
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermQuery); err != nil {
		return nil, err
	}
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
	// P2-5：取不到服务端游标时必须报错。静默回落 0 会让客户端以为「服务端什么都没有」，
	// 进而从 0 全量重拉（大项目直接雪崩）。
	cur, err := s.repo.LatestChangeID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if !hasMore && cursor < cur {
		cursor = cur
	}
	return &SyncChangesResp{Cursor: cursor, Items: items, HasMore: hasMore}, nil
}

// SyncSnapshot 全量分页。
//
// P2-26：改为 **DB 侧分页**。原来是「6 张表全量读进内存 → 拼成一个 rows 切片 → 按 offset/limit 切」，
// 每翻一页都要重扫全量（O(页数 × N)），大项目（上万条接口）翻十几页就明显卡。
// 现在按固定类型顺序（group → api → doc → example → variable_env → variable，与客户端
// typeRank 一致）逐类型「计数 + 只取交集那一段」，单页代价只与 limit 相关。
//
// 引用的 uid（parent_uid / group_uid / api_uid / env_uid）也只针对**本页**的行批量反查，
// 不被页大小以外的数据量拖累。
func (s *Service) SyncSnapshot(ctx context.Context, userID uint64, typ int8, projectID uint64, offset, limit int) (*SyncSnapshotResp, error) {
	// 越权读防护：快照返回项目全量 group/api/doc/response sample 原文
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermQuery); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	// 1) 各类型计数（决定每类在全局序列里的区间）
	counts := make([]int64, 6)
	counter := []func() (int64, error){
		func() (int64, error) { return s.repo.CountGroupsAll(ctx, projectID) },
		func() (int64, error) { return s.repo.CountApisAll(ctx, projectID) },
		func() (int64, error) { return s.repo.CountDocsAll(ctx, projectID) },
		func() (int64, error) { return s.repo.CountApiReturnSamplesAll(ctx, projectID) },
		func() (int64, error) { return s.repo.CountVarEnvsAll(ctx, projectID) },
		func() (int64, error) { return s.repo.CountVariablesAll(ctx, projectID) },
	}
	var total int64
	for i, fn := range counter {
		n, err := fn()
		if err != nil {
			return nil, err
		}
		counts[i] = n
		total += n
	}

	// 2) 逐类型取与本页区间相交的那一段
	loaded, err := s.loadSnapshotSlice(ctx, projectID, offset, limit, counts)
	if err != nil {
		return nil, err
	}
	rows, err := s.snapshotRows(ctx, projectID, loaded)
	if err != nil {
		return nil, err
	}

	// 3) 游标（P2-5：取不到就报错，不回落 0）
	cur, cerr := s.repo.LatestChangeID(ctx, projectID)
	if cerr != nil {
		return nil, cerr
	}
	return &SyncSnapshotResp{
		Cursor:  cur,
		Items:   rows,
		HasMore: int64(offset)+int64(len(rows)) < total,
		Offset:  offset,
	}, nil
}

// snapshotSlice 本页要读的各类型区间。
type snapshotSlice struct {
	groups   []model.Group
	apis     []model.Api
	docs     []model.Doc
	examples []model.ApiReturnSample
	varEnvs  []model.VariableEnv
	vars     []model.Variable
}

// loadSnapshotSlice 按类型顺序把 [offset, offset+limit) 映射到各表的 [局部起点, 条数) 并读取。
func (s *Service) loadSnapshotSlice(ctx context.Context, projectID uint64, offset, limit int, counts []int64) (*snapshotSlice, error) {
	out := &snapshotSlice{}
	skip := int64(offset)  // 还需跳过的全局行数
	remain := int64(limit) // 本页还剩多少条名字额
	take := func(count int64) (int, int) {
		// 返回该类型的 (局部 offset, 条数)
		if remain <= 0 || count <= 0 {
			return 0, 0
		}
		if skip >= count {
			skip -= count
			return 0, 0
		}
		local := int(skip)
		skip = 0
		n := count - int64(local)
		if n > remain {
			n = remain
		}
		remain -= n
		return local, int(n)
	}

	if off, n := take(counts[0]); n > 0 {
		v, err := s.repo.ListGroupsPage(ctx, projectID, off, n)
		if err != nil {
			return nil, err
		}
		out.groups = v
	}
	if off, n := take(counts[1]); n > 0 {
		v, err := s.repo.ListApisPage(ctx, projectID, off, n)
		if err != nil {
			return nil, err
		}
		out.apis = v
	}
	if off, n := take(counts[2]); n > 0 {
		v, err := s.repo.ListDocsPage(ctx, projectID, off, n)
		if err != nil {
			return nil, err
		}
		out.docs = v
	}
	if off, n := take(counts[3]); n > 0 {
		v, err := s.repo.ListApiReturnSamplesPage(ctx, projectID, off, n)
		if err != nil {
			return nil, err
		}
		out.examples = v
	}
	if off, n := take(counts[4]); n > 0 {
		v, err := s.repo.ListVarEnvsPage(ctx, projectID, off, n)
		if err != nil {
			return nil, err
		}
		out.varEnvs = v
	}
	if off, n := take(counts[5]); n > 0 {
		v, err := s.repo.ListVariablesPage(ctx, projectID, off, n)
		if err != nil {
			return nil, err
		}
		out.vars = v
	}
	return out, nil
}

// snapshotRows 把本页各类型的行转成同步条目；引用的 uid 只针对本页批量反查。
func (s *Service) snapshotRows(ctx context.Context, projectID uint64, sl *snapshotSlice) ([]SyncItem, error) {
	// 收集本页需要反查的 id
	groupIDs := make([]uint64, 0, len(sl.groups)+len(sl.apis)+len(sl.docs))
	apiIDs := make([]uint64, 0, len(sl.examples))
	envIDs := make([]uint64, 0, len(sl.vars))
	for _, g := range sl.groups {
		if g.PID > 0 {
			groupIDs = append(groupIDs, g.PID)
		}
	}
	for _, a := range sl.apis {
		if a.GroupID > 0 {
			groupIDs = append(groupIDs, a.GroupID)
		}
	}
	for _, d := range sl.docs {
		if d.GroupID > 0 {
			groupIDs = append(groupIDs, d.GroupID)
		}
	}
	for _, ex := range sl.examples {
		if ex.ApiID > 0 {
			apiIDs = append(apiIDs, ex.ApiID)
		}
	}
	for _, v := range sl.vars {
		if v.EnvID > 0 {
			envIDs = append(envIDs, v.EnvID)
		}
	}
	groupUID, err := s.repo.GroupExtUIDMap(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	apiUID, err := s.repo.ApiExtUIDMap(ctx, apiIDs)
	if err != nil {
		return nil, err
	}
	envUID, err := s.repo.VarEnvExtUIDMap(ctx, envIDs)
	if err != nil {
		return nil, err
	}

	items := make([]SyncItem, 0, len(sl.groups)+len(sl.apis)+len(sl.docs)+len(sl.examples)+len(sl.varEnvs)+len(sl.vars))
	appendItem := func(t string, uid string, rev int64, p SyncPayload) {
		if uid == "" {
			return // 没有 ext_uid 的条目无法参与同步配对
		}
		b, _ := json.Marshal(p)
		items = append(items, SyncItem{Type: t, UID: uid, Op: model.SyncOpUpsert, ItemRev: rev, Payload: b, Deleted: p.Deleted})
	}

	for _, g := range sl.groups {
		p := SyncPayload{Title: g.Title, Priority: g.Priority, Deleted: g.IsDeleted == model.IsDeletedYes}
		p.PID = groupUID[g.PID]
		appendItem(model.SyncTypeGroup, g.ExtUID, g.LastChangeID, p)
	}
	for _, a := range sl.apis {
		p := s.apiSyncPayloadWith(ctx, &a, groupUID)
		p.Deleted = a.IsDeleted == model.IsDeletedYes
		appendItem(model.SyncTypeAPI, a.ExtUID, a.LastChangeID, p)
	}
	for _, d := range sl.docs {
		p := SyncPayload{Title: d.Title, Content: d.Content, Icon: d.Icon, Deleted: d.IsDeleted == model.IsDeletedYes}
		p.GroupUID = groupUID[d.GroupID]
		appendItem(model.SyncTypeDoc, d.ExtUID, d.LastChangeID, p)
	}
	for _, ex := range sl.examples {
		p := s.exampleSyncPayloadWith(&ex, apiUID)
		p.Deleted = ex.IsDeleted == model.IsDeletedYes
		appendItem(model.SyncTypeExample, ex.ExtUID, ex.LastChangeID, p)
	}
	for _, ev := range sl.varEnvs {
		p := SyncPayload{Name: ev.Name, Deleted: ev.IsDeleted == model.IsDeletedYes}
		appendItem(model.SyncTypeVarEnv, ev.ExtUID, ev.LastChangeID, p)
	}
	for _, v := range sl.vars {
		p := s.varSyncPayloadWith(&v, envUID)
		p.Deleted = v.IsDeleted == model.IsDeletedYes
		appendItem(model.SyncTypeVariable, v.ExtUID, v.LastChangeID, p)
	}
	return items, nil
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
		p.APIUID = s.repo.ApiExtUID(ctx, ex.ApiID)
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
		p.EnvUID = s.repo.VarEnvExtUID(ctx, v.EnvID)
	}
	return p
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
func (s *Service) varSyncPayloadWith(v *model.Variable, envUID map[uint64]string) SyncPayload {
	enabled := v.Enabled == model.VarEnabled
	p := SyncPayload{
		Name: v.Name, Value: v.Value,
		Secret: v.IsSecret == model.VarSecret, Enabled: &enabled,
	}
	p.EnvUID = envUID[v.EnvID]
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

// hasDeleteOp 判断批次中是否含删除操作（用于追加 delete 权限位校验）。
func hasDeleteOp(ops []SyncOpIn) bool {
	for _, op := range ops {
		if op.Op == model.SyncOpDelete {
			return true
		}
	}
	return false
}

// SyncPush 批量提交（幂等 + 拓扑序 + 冲突检测）。
func (s *Service) SyncPush(ctx context.Context, userID uint64, typ int8, req *SyncPushReq) (*SyncPushResp, error) {
	if req == nil || req.ProjectID == 0 {
		return nil, errors.New("project_id 必填")
	}
	// 越权写防护：此前无任何权限校验，任意登录用户/PAT 可改写甚至物理删除任意项目数据。
	// 写操作需 update 位；批次中含删除操作时再校验 delete 位。
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	if hasDeleteOp(req.Ops) {
		if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermDelete); err != nil {
			return nil, err
		}
	}
	// 写开关（S14）
	proj, err := s.repo.GetProjectAny(ctx, req.ProjectID)
	if err != nil {
		return nil, err
	}
	// 注：权限位已在函数开头校验过（update 位，含删除时的 delete 位），此处不再重复调用——
	// 同一请求里连做两次相同的权限查询只是多一次 DB 往返（P2-3）。
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
		out = append(out, s.execOp(ctx, userID, req, op, writeDisabled))
	}
	// P2-5：批量写入已经提交，但游标读不出来时不能假装 cursor=0 ——
	// 客户端会以为自己的游标最旧，下一轮从 0 全量重拉。宁可让本次响应失败，让它重试。
	cur, err := s.repo.LatestChangeID(ctx, req.ProjectID)
	if err != nil {
		return nil, err
	}
	return &SyncPushResp{Results: out, Cursor: cur}, nil
}

// errOpRollback 只用于在事务回调里请求回滚：业务结果已经写进 res，不需要真的返回错误。
var errOpRollback = errors.New("sync: op rejected, rollback")

// execOp 在**单个事务**内应用一条 op：实体写入 + 变更流水 + 修订号 + 幂等记录。
//
// P1-1：此前这三步各自独立提交。第 2 步失败会留下「数据已改但没有流水」的条目
// （客户端永远拉不到），第 3 步失败会留下「没有幂等记录」的条目（重放时被再执行一遍）。
// 现在任一步失败整条 op 回滚，客户端拿到 rejected 后重试是安全的。
//
// 业务性拒绝（conflict / rejected / readonly）同样回滚：它们已经写了一半的实体必须撤销，
// 只有 ok 与 conflict 需要留下幂等记录（conflict 的重复推送应稳定返回 conflict，而不是重算）。
func (s *Service) execOp(ctx context.Context, userID uint64, req *SyncPushReq, op SyncOpIn, writeDisabled bool) SyncOpOut {
	var res SyncOpOut
	err := s.repo.Transaction(ctx, func(tx *repository.Repository) error {
		res = s.applyOp(ctx, tx, userID, req, op, writeDisabled)
		if res.Status != "ok" && res.Status != "conflict" {
			return errOpRollback
		}
		if err := tx.SaveSyncOp(ctx, &model.SyncOp{
			OpID: op.OpID, ProjectID: req.ProjectID,
			Status: res.Status, ItemRev: res.ItemRev, Reason: res.Reason,
		}); err != nil {
			res = SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "idempotency record failed"}
			return errOpRollback
		}
		return nil
	})
	if err != nil && !errors.Is(err, errOpRollback) {
		// 事务本身失败（提交出错、锁超时等）：不能谎报成功，让客户端重试。
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "transaction failed"}
	}
	return res
}

// applyOp 处理单条 push。
func (s *Service) applyOp(ctx context.Context, r *repository.Repository, userID uint64, req *SyncPushReq, op SyncOpIn, writeDisabled bool) SyncOpOut {
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
		e, lerr := r.FindGroupByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeAPI:
		e, lerr := r.FindApiByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeDoc:
		e, lerr := r.FindDocByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeExample:
		e, lerr := r.FindApiReturnSampleByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeVarEnv:
		e, lerr := r.FindVarEnvByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeVariable:
		e, lerr := r.FindVariableByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
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
		return s.applyDelete(ctx, r, userID, req, op)
	}
	return s.applyUpsert(ctx, r, userID, req, op)
}

// lookupRejected 查询实体出错时返回 rejected（P1-3）：
// 只有确实不存在才允许走新建分支；DB 错误被当成不存在会导致重复创建或整条覆盖。
func lookupRejected(op SyncOpIn, err error) (SyncOpOut, bool) {
	if err == nil || errors.Is(err, repository.ErrNotFound) {
		return SyncOpOut{}, false
	}
	return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "lookup failed"}, true
}

func (s *Service) applyUpsert(ctx context.Context, r *repository.Repository, userID uint64, req *SyncPushReq, op SyncOpIn) SyncOpOut {
	var p SyncPayload
	if len(op.Payload) > 0 {
		// P0-3：解析失败必须拒收（rejected），不能用零值覆盖存量数据——
		// 否则一次畸形 payload 就会把服务端上的标题/内容/请求参数静默清空。
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "bad payload"}
		}
	}
	device := req.DeviceID
	if req.DeviceName != "" {
		device = req.DeviceName
	}

	switch op.Type {
	case model.SyncTypeGroup:
		e, lerr := r.FindGroupByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			e = &model.Group{ProjectID: req.ProjectID, Type: model.GroupTypeAPI}
		}
		e.ExtUID = op.UID
		e.Title = p.Title
		e.Priority = p.Priority
		if p.PID != "" {
			if pg, _ := r.FindGroupByExtUID(ctx, req.ProjectID, p.PID); pg != nil {
				e.PID = pg.ID
			} else {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
		} else {
			e.PID = 0
		}
		var err error
		if e.ID == 0 {
			err = r.CreateGroup(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateGroup(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeGroup, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}

	case model.SyncTypeAPI:
		e, lerr := r.FindApiByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			e = &model.Api{ProjectID: req.ProjectID}
		}
		e.ExtUID = op.UID
		e.ApiName = p.Name
		e.HttpMethodType = p.Method
		e.URL = p.URL
		if p.GroupUID != "" {
			if g, _ := r.FindGroupByExtUID(ctx, req.ProjectID, p.GroupUID); g != nil {
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
			err = r.CreateApi(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateApi(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeAPI, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}

	case model.SyncTypeDoc:
		e, lerr := r.FindDocByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			e = &model.Doc{ProjectID: req.ProjectID, UserID: userID}
		}
		e.ExtUID = op.UID
		e.Title = p.Title
		e.Content = p.Content
		e.Icon = p.Icon
		if p.GroupUID != "" {
			if g, _ := r.FindGroupByExtUID(ctx, req.ProjectID, p.GroupUID); g != nil {
				e.GroupID = g.ID
			} else {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
		} else {
			e.GroupID = 0
		}
		var err error
		if e.ID == 0 {
			err = r.CreateDoc(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateDoc(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeDoc, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}

	case model.SyncTypeExample:
		e, lerr := r.FindApiReturnSampleByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			e = &model.ApiReturnSample{ProjectID: req.ProjectID}
		}
		e.ExtUID = op.UID
		e.Name = p.Name
		e.SampleText = p.Sample
		e.Status = p.Status
		e.ContentType = p.ContentType
		e.Seq = p.Seq
		if p.APIUID != "" {
			if a, _ := r.FindApiByExtUID(ctx, req.ProjectID, p.APIUID); a != nil {
				e.ApiID = a.ID
			} else {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
		}
		var err error
		if e.ID == 0 {
			err = r.CreateApiReturnSample(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateApiReturnSample(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeExample, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}

	case model.SyncTypeVarEnv:
		// 环境：Name 复用载荷字段；仅建/改同步身份与名称，颜色/描述等 Web 侧属性不覆盖。
		e, lerr := r.FindVarEnvByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			e = &model.VariableEnv{ProjectID: req.ProjectID}
		}
		e.ExtUID = op.UID
		if p.Name != "" {
			e.Name = p.Name
		}
		var err error
		if e.ID == 0 {
			err = r.CreateVariableEnv(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateVariableEnv(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeVarEnv, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}

	case model.SyncTypeVariable:
		// 变量：项目级、挂到 env_uid 对应环境；Value 原样存（secret 为 enc:v1: 密文，服务端不解析）。
		e, lerr := r.FindVariableByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
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
			env, _ := r.FindVarEnvByExtUID(ctx, req.ProjectID, p.EnvUID)
			if env == nil {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
			e.EnvID = env.ID
		}
		var err error
		if e.ID == 0 {
			err = r.CreateVariable(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateVariable(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeVariable, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}
	}
	return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
}

func (s *Service) applyDelete(ctx context.Context, r *repository.Repository, userID uint64, req *SyncPushReq, op SyncOpIn) SyncOpOut {
	device := req.DeviceID
	if req.DeviceName != "" {
		device = req.DeviceName
	}
	var id uint64
	switch op.Type {
	case model.SyncTypeGroup:
		e, lerr := r.FindGroupByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteGroup(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeAPI:
		e, lerr := r.FindApiByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteApi(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeDoc:
		e, lerr := r.FindDocByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteDoc(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeExample:
		e, lerr := r.FindApiReturnSampleByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteApiReturnSample(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeVarEnv:
		e, lerr := r.FindVarEnvByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteVariableEnv(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeVariable:
		e, lerr := r.FindVariableByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteVariable(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	default:
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
	}
	cid, err := s.appendChangeTx(ctx, r, op.Type, op.UID, req.ProjectID, id, model.SyncOpDelete, userID, device)
	if err != nil {
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
	}
	return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}
}
