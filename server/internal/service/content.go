package service

import (
	"context"
	"encoding/json"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/page"
	"apidoc/internal/repository"
)

// ---- 分组 ----

// CreateGroup 新建分组。
func (s *Service) CreateGroup(ctx context.Context, userID uint64, typ int8, req dto.GroupCreateReq) (*model.Group, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermCreate); err != nil {
		return nil, err
	}
	g := &model.Group{ProjectID: req.ProjectID, PID: req.PID, Title: req.Title, Type: req.Type, Priority: req.Priority}
	if err := s.repo.CreateGroup(ctx, g); err != nil {
		return nil, err
	}
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
	if err := s.repo.UpdateGroup(ctx, g); err != nil {
		return nil, err
	}
	return g, nil
}

// ListGroups 列出分组（type<0 不过滤类型）。
func (s *Service) ListGroups(ctx context.Context, projectID uint64, typ int8) ([]model.Group, error) {
	return s.repo.ListGroups(ctx, projectID, typ)
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
	return s.repo.DeleteGroup(ctx, id)
}

// ---- 接口文档 ----

// CreateApi 新建接口；应用字段映射后落库。
func (s *Service) CreateApi(ctx context.Context, userID uint64, typ int8, req dto.ApiCreateReq) (*model.Api, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermCreate); err != nil {
		return nil, err
	}
	a := apiFromCreate(req)
	s.applyFieldMapping(ctx, req.ProjectID, a)
	if err := s.repo.CreateApi(ctx, a); err != nil {
		return nil, err
	}
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
	updated := apiFromCreate(req.ApiCreateReq)
	updated.ID = a.ID
	updated.BaseModel = a.BaseModel
	s.applyFieldMapping(ctx, a.ProjectID, updated)
	if err := s.repo.UpdateApi(ctx, updated); err != nil {
		return nil, err
	}
	return updated, nil
}

// GetApi 接口详情。
func (s *Service) GetApi(ctx context.Context, id uint64) (*model.Api, error) {
	return s.repo.GetApi(ctx, id)
}

// ListApis 分页查询接口。
func (s *Service) ListApis(ctx context.Context, q dto.ApiListQuery) (page.Result[model.Api], error) {
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
	return s.repo.DeleteApi(ctx, id)
}

// RestoreApi 恢复接口。
func (s *Service) RestoreApi(ctx context.Context, userID uint64, typ int8, id uint64) error {
	return s.repo.RestoreApi(ctx, id)
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
	d := &model.Doc{ProjectID: req.ProjectID, GroupID: req.GroupID, UserID: userID, Title: req.Title, Content: req.Content}
	if err := s.repo.CreateDoc(ctx, d); err != nil {
		return nil, err
	}
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
	if req.Title != "" {
		d.Title = req.Title
	}
	d.Content = req.Content
	d.GroupID = req.GroupID
	d.IsTop = req.IsTop
	if err := s.repo.UpdateDoc(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// GetDoc 文档详情（浏览数 +1）。
func (s *Service) GetDoc(ctx context.Context, id uint64) (*model.Doc, error) {
	d, err := s.repo.GetDoc(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.repo.IncrDocView(ctx, id)
	d.ViewCount++
	return d, nil
}

// ListDocs 分页查询文档。deleted=1 查回收站。
func (s *Service) ListDocs(ctx context.Context, projectID, groupID uint64, keyword string, deleted int8, q page.Query) (page.Result[model.Doc], error) {
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
	return s.repo.DeleteDoc(ctx, id)
}

// RestoreDoc 恢复文档。
func (s *Service) RestoreDoc(ctx context.Context, id uint64) error {
	return s.repo.RestoreDoc(ctx, id)
}

// LikeDoc 点赞。
func (s *Service) LikeDoc(ctx context.Context, id uint64) error {
	if _, err := s.repo.GetDoc(ctx, id); err != nil {
		return err
	}
	return s.repo.IncrDocLike(ctx, id)
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

// ListFieldMappings 列出项目字段映射。
func (s *Service) ListFieldMappings(ctx context.Context, projectID uint64) ([]model.FieldMapping, error) {
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
