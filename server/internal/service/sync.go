package service

import (
	"context"
	"encoding/json"

	"apidoc/internal/model"

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
		Features:         []string{"sync", "pat", "change_log", "tombstone", "op_id", "field_mapping"},
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

// clampChangeLimit 增量拉取的 limit 收敛（审核 H-2）：与 repository 的上界保持一致（1000）。
func clampChangeLimit(limit int) int {
	switch {
	case limit <= 0:
		return 200
	case limit > 1000:
		return 1000
	default:
		return limit
	}
}

// clampSnapshotLimit 快照分页的 limit 收敛（审核 H-3）：上限 500，默认 200。
func clampSnapshotLimit(limit int) int {
	if limit <= 0 || limit > 500 {
		return 200
	}
	return limit
}

// SyncChanges 增量拉取。
func (s *Service) SyncChanges(ctx context.Context, userID uint64, typ int8, projectID uint64, since int64, limit int) (*SyncChangesResp, error) {
	// 越权读防护：变更流含全量接口定义（URL/请求头/请求体/响应样例）
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermQuery); err != nil {
		return nil, err
	}
	// 审核 H-2：limit 必须在入口收敛，且与 repository 同一上界（1000）。
	// 否则 repo 静默把 >1000 改成 200，而 hasMore 仍按原 limit 判定 → 客户端拿到
	// 「cursor=最新、HasMore=false」却没收到中间数千条变更（无告警的数据丢失）。
	limit = clampChangeLimit(limit)
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
// 现在按固定类型顺序（group → api → doc → example → variable_env → variable → field_mapping，与客户端
// typeRank 一致）逐类型「计数 + 只取交集那一段」，单页代价只与 limit 相关。
//
// 引用的 uid（parent_uid / group_uid / api_uid / env_uid）也只针对**本页**的行批量反查，
// 不被页大小以外的数据量拖累。
func (s *Service) SyncSnapshot(ctx context.Context, userID uint64, typ int8, projectID uint64, offset, limit int) (*SyncSnapshotResp, error) {
	// 越权读防护：快照返回项目全量 group/api/doc/response sample 原文
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermQuery); err != nil {
		return nil, err
	}
	// 审核 H-3：快照分页同样收敛上界，避免 ?limit=1000000 把整表读进内存（内存放大 DoS）。
	limit = clampSnapshotLimit(limit)
	if offset < 0 {
		offset = 0
	}

	// 1) 各类型计数（决定每类在全局序列里的区间）
	counts := make([]int64, 7)
	counter := []func() (int64, error){
		func() (int64, error) { return s.repo.CountGroupsAll(ctx, projectID) },
		func() (int64, error) { return s.repo.CountApisAll(ctx, projectID) },
		func() (int64, error) { return s.repo.CountDocsAll(ctx, projectID) },
		func() (int64, error) { return s.repo.CountApiReturnSamplesAll(ctx, projectID) },
		func() (int64, error) { return s.repo.CountVarEnvsAll(ctx, projectID) },
		func() (int64, error) { return s.repo.CountVariablesAll(ctx, projectID) },
		func() (int64, error) { return s.repo.CountFieldMappingsAll(ctx, projectID) },
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
	groups        []model.Group
	apis          []model.Api
	docs          []model.Doc
	examples      []model.ApiReturnSample
	varEnvs       []model.VariableEnv
	vars          []model.Variable
	fieldMappings []model.FieldMapping
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
	if off, n := take(counts[6]); n > 0 {
		v, err := s.repo.ListFieldMappingsPage(ctx, projectID, off, n)
		if err != nil {
			return nil, err
		}
		out.fieldMappings = v
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

	items := make([]SyncItem, 0, len(sl.groups)+len(sl.apis)+len(sl.docs)+len(sl.examples)+len(sl.varEnvs)+len(sl.vars)+len(sl.fieldMappings))
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
	for _, fm := range sl.fieldMappings {
		p := SyncPayload{Field: fm.Field, FieldType: fm.Type, Description: fm.Description}
		p.Deleted = fm.IsDeleted == model.IsDeletedYes
		appendItem(model.SyncTypeFieldMapping, fm.ExtUID, fm.LastChangeID, p)
	}
	return items, nil
}
