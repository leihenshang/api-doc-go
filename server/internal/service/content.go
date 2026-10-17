package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sort"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/page"
	"apidoc/internal/repository"
)

// ---- 分组 ----

// recordChange 在 Web 写路径落变更流水（S3：覆盖 Web 与 sync 两个入口）。
// ext_uid 为空时自动生成，保证客户端能按 uid 配对。
func (s *Service) recordChange(entityType, extUID string, projectID uint64, id uint64, op string, actor uint64) {
	if extUID == "" {
		return
	}
	cid, err := s.repo.AppendChange(nil, projectID, entityType, extUID, op, actor, "web")
	if err != nil {
		return
	}
	_ = s.repo.SetLastChange(nil, entityType, id, cid)
}

// ensureExtUID 条目缺 uid 时补一个（Web 新建也分配，便于后续同步）。
func ensureExtUID(cur string) string {
	if cur != "" {
		return cur
	}
	return newExtUID()
}

// newExtUID 生成客户端风格的 UUIDv4 字符串。
func newExtUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// CreateGroup 新建分组。
func (s *Service) CreateGroup(ctx context.Context, userID uint64, typ int8, req dto.GroupCreateReq) (*model.Group, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermCreate); err != nil {
		return nil, err
	}
	g := &model.Group{ProjectID: req.ProjectID, PID: req.PID, Title: req.Title, Type: req.Type, Priority: req.Priority}
	g.ExtUID = ensureExtUID("")
	if err := s.repo.CreateGroup(ctx, g); err != nil {
		return nil, err
	}
	s.recordChange(model.SyncTypeGroup, g.ExtUID, g.ProjectID, g.ID, model.SyncOpUpsert, userID)
	return g, nil
}

// UpdateGroup 更新分组。
func (s *Service) UpdateGroup(ctx context.Context, userID uint64, typ int8, req dto.GroupUpdateReq) (*model.Group, error) {
	g, err := s.repo.GetGroup(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, g.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	if req.Title != "" {
		g.Title = req.Title
	}
	g.PID = req.PID
	g.Priority = req.Priority
	g.ExtUID = ensureExtUID(g.ExtUID)
	if err := s.repo.UpdateGroup(ctx, g); err != nil {
		return nil, err
	}
	s.recordChange(model.SyncTypeGroup, g.ExtUID, g.ProjectID, g.ID, model.SyncOpUpsert, userID)
	return g, nil
}

// ListGroups 列出分组（groupType<0 不过滤类型）。P0-1：需项目读权限。
func (s *Service) ListGroups(ctx context.Context, userID uint64, userType int8, projectID uint64, groupType int8) ([]model.Group, error) {
	if err := s.ensureProjectPermission(ctx, userID, userType, projectID, model.PermQuery); err != nil {
		return nil, err
	}
	return s.repo.ListGroups(ctx, projectID, groupType)
}

// ListDocGroups 文档分组树（B19 / Web 展示）：返回「含文档的分组 + 其祖先链」，
// 每项带 doc_count；不再按 type 过滤（客户端文档可与接口共用分组）。
func (s *Service) ListDocGroups(ctx context.Context, userID uint64, typ int8, projectID uint64) ([]dto.DocGroupItem, int64, error) {
	if projectID == 0 {
		return nil, 0, newBiz(errcode.CodeParamError, "缺少 project_id")
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermQuery); err != nil {
		return nil, 0, err
	}
	counts, ungrouped, err := s.repo.DocGroupCounts(ctx, projectID)
	if err != nil {
		return nil, 0, err
	}
	if len(counts) == 0 {
		return nil, ungrouped, nil
	}
	groups, err := s.repo.ListGroups(ctx, projectID, -1)
	if err != nil {
		return nil, 0, err
	}
	// 需要展示的分组 = 有文档的分组 + 它们的祖先（否则树会断链）
	keep := make(map[uint64]bool, len(counts))
	byID := make(map[uint64]model.Group, len(groups))
	for _, g := range groups {
		byID[g.ID] = g
	}
	for id := range counts {
		cur := id
		for cur != 0 {
			if keep[cur] {
				break
			}
			keep[cur] = true
			parent, ok := byID[cur]
			if !ok || parent.PID == cur {
				break
			}
			cur = parent.PID
		}
	}
	out := make([]dto.DocGroupItem, 0, len(keep))
	for _, g := range groups {
		if !keep[g.ID] {
			continue
		}
		out = append(out, dto.DocGroupItem{
			ID: g.ID, PID: g.PID, Title: g.Title, Type: g.Type,
			Priority: g.Priority, DocCount: counts[g.ID],
		})
	}
	return out, ungrouped, nil
}

// DeleteGroup 删除分组。
func (s *Service) DeleteGroup(ctx context.Context, userID uint64, typ int8, id uint64) error {
	g, err := s.repo.GetGroup(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, g.ProjectID, model.PermDelete); err != nil {
		return err
	}
	if err := s.repo.DeleteGroup(ctx, id); err != nil {
		return err
	}
	s.recordChange(model.SyncTypeGroup, g.ExtUID, g.ProjectID, g.ID, model.SyncOpDelete, userID)
	return nil
}

// ---- 接口文档 ----

// CreateApi 新建接口；应用字段映射后落库。
func (s *Service) CreateApi(ctx context.Context, userID uint64, typ int8, req dto.ApiCreateReq) (*model.Api, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermCreate); err != nil {
		return nil, err
	}
	a := apiFromCreate(req)
	a.ExtUID = ensureExtUID("")
	s.applyFieldMapping(ctx, req.ProjectID, a)
	if err := s.repo.CreateApi(ctx, a); err != nil {
		return nil, err
	}
	s.recordChange(model.SyncTypeAPI, a.ExtUID, a.ProjectID, a.ID, model.SyncOpUpsert, userID)
	s.recordOp(ctx, req.ProjectID, userID, a.ID, model.LogObjectAPI, "create", "接口:"+a.ApiName)
	return a, nil
}

// UpdateApi 更新接口。
func (s *Service) UpdateApi(ctx context.Context, userID uint64, typ int8, req dto.ApiUpdateReq) (*model.Api, error) {
	a, err := s.repo.GetApi(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, a.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	// 乐观锁：客户端载入后若他人已保存，则拒绝覆盖（未传 expected_update_time 时不做校验）。
	if err := ensureNotStale(req.ExpectedUpdateTime, a.UpdateTime, "接口"); err != nil {
		return nil, err
	}
	updated := apiFromCreate(req.ApiCreateReq)
	updated.ID = a.ID
	updated.BaseModel = a.BaseModel
	updated.ExtUID = ensureExtUID(a.ExtUID)
	updated.LastChangeID = a.LastChangeID
	s.applyFieldMapping(ctx, a.ProjectID, updated)
	if err := s.repo.UpdateApi(ctx, updated); err != nil {
		return nil, err
	}
	s.recordChange(model.SyncTypeAPI, updated.ExtUID, updated.ProjectID, updated.ID, model.SyncOpUpsert, userID)
	return updated, nil
}

// GetApi 接口详情。P0-1：需所属项目读权限（此前任意登录用户可按 id 枚举）。
func (s *Service) GetApi(ctx context.Context, userID uint64, userType int8, id uint64) (*model.Api, error) {
	a, err := s.repo.GetApi(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, userType, a.ProjectID, model.PermQuery); err != nil {
		return nil, err
	}
	return a, nil
}

// ListApis 分页查询接口。P0-1：需项目读权限。
func (s *Service) ListApis(ctx context.Context, userID uint64, userType int8, q dto.ApiListQuery) (page.Result[model.Api], error) {
	if err := s.ensureProjectPermission(ctx, userID, userType, q.ProjectID, model.PermQuery); err != nil {
		return page.Result[model.Api]{}, err
	}
	pq := page.Query{PageNum: q.PageNum, PageSize: q.PageSize}
	pq.Normalize()
	items, total, err := s.repo.ListApis(ctx, q.ProjectID, q.GroupID, q.Deleted, q.Keyword, pq)
	if err != nil {
		return page.Result[model.Api]{}, err
	}
	return page.NewResult(items, total, pq), nil
}

// DeleteApi 软删除接口。
func (s *Service) DeleteApi(ctx context.Context, userID uint64, typ int8, id uint64) error {
	a, err := s.repo.GetApi(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, a.ProjectID, model.PermDelete); err != nil {
		return err
	}
	if err := s.repo.DeleteApi(ctx, id); err != nil {
		return err
	}
	s.recordChange(model.SyncTypeAPI, a.ExtUID, a.ProjectID, a.ID, model.SyncOpDelete, userID)
	return nil
}

// collectParents 汇总条目所属的项目 id 与分组 id（去重）。
func collectParents[T any](items []T, pick func(T) (uint64, uint64)) ([]uint64, []uint64) {
	projects := make([]uint64, 0, len(items))
	groups := make([]uint64, 0, len(items))
	seen := map[uint64]struct{}{}
	for _, it := range items {
		pid, gid := pick(it)
		if _, ok := seen[pid]; !ok {
			seen[pid] = struct{}{}
			projects = append(projects, pid)
		}
		if gid > 0 {
			groups = append(groups, gid)
		}
	}
	return projects, groups
}

// ensureProjectsUpdate / ensureProjectsPerm 批量校验多个项目的权限（任一不通过即拒绝）。
func (s *Service) ensureProjectsUpdate(ctx context.Context, userID uint64, typ int8, projectIDs []uint64) error {
	return s.ensureProjectsPerm(ctx, userID, typ, projectIDs, model.PermUpdate)
}

func (s *Service) ensureProjectsPerm(ctx context.Context, userID uint64, typ int8, projectIDs []uint64, need int8) error {
	for _, pid := range projectIDs {
		if err := s.ensureProjectPermission(ctx, userID, typ, pid, need); err != nil {
			return err
		}
	}
	return nil
}

// deletedGroupChain 从 from 出发沿 p_id 向上收集「已删除」的分组 id（去重升序），用于恢复时补齐祖先链。
func deletedGroupChain(groups []model.Group, from []uint64) []uint64 {
	byID := make(map[uint64]model.Group, len(groups))
	for _, g := range groups {
		byID[g.ID] = g
	}
	need := map[uint64]struct{}{}
	for _, id := range from {
		cur, guard := id, 0
		for cur != 0 && guard < 32 {
			g, ok := byID[cur]
			if !ok {
				break
			}
			if g.IsDeleted == 1 {
				need[g.ID] = struct{}{}
			}
			cur = g.PID
			guard++
		}
	}
	out := make([]uint64, 0, len(need))
	for id := range need {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// apiFromCreate 将 DTO 映射为实体（原始 JSON 存为文本列）。
func apiFromCreate(req dto.ApiCreateReq) *model.Api {
	return &model.Api{
		ProjectID: req.ProjectID, GroupID: req.GroupID, GroupIDSecond: req.GroupIDSecond,
		ApiName: req.ApiName, ProtocolType: req.ProtocolType, HttpMethodType: req.HttpMethodType,
		URL: req.URL, Description: req.Description, ObjectName: req.ObjectName,
		FunctionName: req.FunctionName, DevelopLanguage: req.DevelopLanguage, HttpReturnType: req.HttpReturnType,
		HttpRequestHeader: rawString(req.HttpRequestHeader), HttpRequestParams: rawString(req.HttpRequestParams),
		HttpReturnParams: rawString(req.HttpReturnParams), HttpReturnSample: rawString(req.HttpReturnSample),
		// 客户端富字段：原样透传（保存须带回原值，避免整条覆盖清空）。
		HttpRequestBodyType: req.HttpRequestBodyType, HttpRequestBody: req.HttpRequestBody,
		HttpRequestBodyForm: req.HttpRequestBodyForm, HttpRequestBodyDescs: req.HttpRequestBodyDescs,
		HttpAuth: req.HttpAuth, RequestSettings: req.RequestSettings,
		HttpScript: req.HttpScript, ScriptVars: req.ScriptVars,
		ScriptAsserts: req.ScriptAsserts, HttpReturnContentType: req.HttpReturnContentType,
	}
}

// applyFieldMapping 依据项目字段字典回填返回参数的 type/description。
func (s *Service) applyFieldMapping(ctx context.Context, projectID uint64, a *model.Api) {
	if a.HttpReturnParams == "" {
		return
	}
	var params []map[string]any
	if err := json.Unmarshal([]byte(a.HttpReturnParams), &params); err != nil {
		return
	}
	changed := false
	for _, p := range params {
		field, _ := p["fieldName"].(string)
		if field == "" {
			continue
		}
		fm, err := s.repo.FieldMappingByField(ctx, projectID, field)
		if err != nil {
			continue
		}
		p["type"] = fm.Type
		if d, _ := p["description"].(string); d == "" {
			p["description"] = fm.Description
		}
		changed = true
	}
	if changed {
		if b, err := json.Marshal(params); err == nil {
			a.HttpReturnParams = string(b)
		}
	}
}

// rawString 将原始 JSON 转为字符串（空则返回空串）。
func rawString(r json.RawMessage) string { return string(r) }

// ---- 文档 ----

// CreateDoc 新建文档。
func (s *Service) CreateDoc(ctx context.Context, userID uint64, typ int8, req dto.DocCreateReq) (*model.Doc, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermCreate); err != nil {
		return nil, err
	}
	d := &model.Doc{
		ProjectID: req.ProjectID, GroupID: req.GroupID, UserID: userID,
		Title: req.Title, Content: req.Content, Icon: req.Icon,
	}
	d.ExtUID = ensureExtUID("")
	if err := s.repo.CreateDoc(ctx, d); err != nil {
		return nil, err
	}
	s.recordChange(model.SyncTypeDoc, d.ExtUID, d.ProjectID, d.ID, model.SyncOpUpsert, userID)
	s.recordOp(ctx, req.ProjectID, userID, d.ID, model.LogObjectDoc, "create", "文档:"+d.Title)
	return d, nil
}

// UpdateDoc 更新文档。
func (s *Service) UpdateDoc(ctx context.Context, userID uint64, typ int8, req dto.DocUpdateReq) (*model.Doc, error) {
	d, err := s.repo.GetDoc(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, d.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	// 乐观锁：与接口保存一致（未传 expected_update_time 时不做校验）。
	if err := ensureNotStale(req.ExpectedUpdateTime, d.UpdateTime, "文档"); err != nil {
		return nil, err
	}
	if req.Title != "" {
		d.Title = req.Title
	}
	d.Content = req.Content
	d.GroupID = req.GroupID
	d.IsTop = req.IsTop
	// 图标以请求值为准：空串表示清除图标。
	d.Icon = req.Icon
	d.ExtUID = ensureExtUID(d.ExtUID)
	if err := s.repo.UpdateDoc(ctx, d); err != nil {
		return nil, err
	}
	s.recordChange(model.SyncTypeDoc, d.ExtUID, d.ProjectID, d.ID, model.SyncOpUpsert, userID)
	return d, nil
}

// GetDoc 文档详情（浏览数 +1）。P0-1：需所属项目读权限。
func (s *Service) GetDoc(ctx context.Context, userID uint64, userType int8, id uint64) (*model.Doc, error) {
	d, err := s.repo.GetDoc(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, userType, d.ProjectID, model.PermQuery); err != nil {
		return nil, err
	}
	_ = s.repo.IncrDocView(ctx, id)
	d.ViewCount++
	return d, nil
}

// ListDocs 分页查询文档。deleted=1 查回收站。P0-1：需项目读权限。
func (s *Service) ListDocs(ctx context.Context, userID uint64, userType int8, projectID, groupID uint64, keyword string, deleted int8, q page.Query) (page.Result[model.Doc], error) {
	if err := s.ensureProjectPermission(ctx, userID, userType, projectID, model.PermQuery); err != nil {
		return page.Result[model.Doc]{}, err
	}
	q.Normalize()
	items, total, err := s.repo.ListDocs(ctx, projectID, groupID, keyword, deleted, q)
	if err != nil {
		return page.Result[model.Doc]{}, err
	}
	return page.NewResult(items, total, q), nil
}

// DeleteDoc 软删除文档。
func (s *Service) DeleteDoc(ctx context.Context, userID uint64, typ int8, id uint64) error {
	d, err := s.repo.GetDoc(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, d.ProjectID, model.PermDelete); err != nil {
		return err
	}
	if err := s.repo.DeleteDoc(ctx, id); err != nil {
		return err
	}
	s.recordChange(model.SyncTypeDoc, d.ExtUID, d.ProjectID, d.ID, model.SyncOpDelete, userID)
	return nil
}

// LikeDoc 点赞。
func (s *Service) LikeDoc(ctx context.Context, id uint64) error {
	if _, err := s.repo.GetDoc(ctx, id); err != nil {
		return err
	}
	return s.repo.IncrDocLike(ctx, id)
}

// ---- 拖拽排序 ----

// SortNodes 批量保存拖拽后的顺序：分组改 p_id/priority；接口、文档改 group_id/priority。
func (s *Service) SortNodes(ctx context.Context, userID uint64, typ int8, req dto.SortReq) error {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermUpdate); err != nil {
		return err
	}
	for _, it := range req.Items {
		if err := s.sortNode(ctx, req, it); err != nil {
			return err
		}
	}
	return nil
}

// sortNode 依据 kind 落库单个节点的新父级与同级顺序。
func (s *Service) sortNode(ctx context.Context, req dto.SortReq, it dto.SortItem) error {
	switch req.Kind {
	case "group":
		return s.sortGroup(ctx, req.ProjectID, it)
	case "api":
		a, err := s.repo.GetApi(ctx, it.ID)
		if err != nil {
			return err
		}
		if a.ProjectID != req.ProjectID {
			return newBiz(errcode.CodeParamError, "接口不属于该项目")
		}
		return s.repo.UpdateApiSort(ctx, it.ID, it.ParentID, it.Priority)
	case "doc":
		d, err := s.repo.GetDoc(ctx, it.ID)
		if err != nil {
			return err
		}
		if d.ProjectID != req.ProjectID {
			return newBiz(errcode.CodeParamError, "文档不属于该项目")
		}
		return s.repo.UpdateDocSort(ctx, it.ID, it.ParentID, it.Priority)
	default:
		return newBiz(errcode.CodeParamError, "未知排序类型")
	}
}

// sortGroup 移动/排序分组；禁止挂到自身或后代下，避免成环。
func (s *Service) sortGroup(ctx context.Context, projectID uint64, it dto.SortItem) error {
	g, err := s.repo.GetGroup(ctx, it.ID)
	if err != nil {
		return err
	}
	if g.ProjectID != projectID {
		return newBiz(errcode.CodeParamError, "分组不属于该项目")
	}
	cyclic, err := s.groupMoveCyclic(ctx, it.ID, it.ParentID)
	if err != nil {
		return err
	}
	if cyclic {
		return newBiz(errcode.CodeParamError, "不能把分组移动到其自身或子分组下")
	}
	return s.repo.UpdateGroupSort(ctx, it.ID, it.ParentID, it.Priority)
}

// groupMoveCyclic 判断把 id 挂到 parentID 下是否成环（parentID 为 id 自身或其后代）。
func (s *Service) groupMoveCyclic(ctx context.Context, id, parentID uint64) (bool, error) {
	for pid := parentID; pid > 0; {
		if pid == id {
			return true, nil
		}
		g, err := s.repo.GetGroup(ctx, pid)
		if err != nil {
			return false, nil // 父级不存在，视为无环
		}
		pid = g.PID
	}
	return false, nil
}

// ---- 字段映射 ----

// CreateFieldMapping 新建字段映射。
func (s *Service) CreateFieldMapping(ctx context.Context, userID uint64, typ int8, req dto.FieldMappingReq) (*model.FieldMapping, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermCreate); err != nil {
		return nil, err
	}
	if _, err := s.repo.FieldMappingByField(ctx, req.ProjectID, req.Field); err == nil {
		return nil, newBiz(errcode.CodeFailed, "字段重复")
	}
	fm := &model.FieldMapping{ProjectID: req.ProjectID, Field: req.Field, Type: req.Type, Description: req.Description}
	if err := s.repo.CreateFieldMapping(ctx, fm); err != nil {
		return nil, err
	}
	return fm, nil
}

// ListFieldMappings 列出项目字段映射。P0-1：需项目读权限。
func (s *Service) ListFieldMappings(ctx context.Context, userID uint64, userType int8, projectID uint64) ([]model.FieldMapping, error) {
	if err := s.ensureProjectPermission(ctx, userID, userType, projectID, model.PermQuery); err != nil {
		return nil, err
	}
	return s.repo.ListFieldMappings(ctx, projectID)
}

// UpdateFieldMapping 更新字段映射。
func (s *Service) UpdateFieldMapping(ctx context.Context, userID uint64, typ int8, req dto.FieldMappingReq) (*model.FieldMapping, error) {
	fm, err := s.repo.GetFieldMapping(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, fm.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	fm.Field, fm.Type, fm.Description = req.Field, req.Type, req.Description
	if err := s.repo.UpdateFieldMapping(ctx, fm); err != nil {
		return nil, err
	}
	return fm, nil
}

// DeleteFieldMapping 删除字段映射。
func (s *Service) DeleteFieldMapping(ctx context.Context, userID uint64, typ int8, id uint64) error {
	fm, err := s.repo.GetFieldMapping(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, fm.ProjectID, model.PermDelete); err != nil {
		return err
	}
	return s.repo.DeleteFieldMapping(ctx, id)
}

// 保证 repository 被引用（避免后续重构误删依赖）。
var _ = repository.ErrNotFound
