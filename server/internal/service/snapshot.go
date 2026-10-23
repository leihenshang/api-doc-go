package service

// 内容快照（RM-05）：接口/文档每次 Web 保存后留一份完整载荷，
// 提供历史列表、与当前内容的字段级 diff、回滚到历史快照。

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"time"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

const revisionListLimit = 50

// saveSnapshot 写入一份快照；失败只记日志语义（不阻断保存），revision 为 change_log id。
func (s *Service) saveSnapshot(ctx context.Context, projectID uint64, entityType string, entityID uint64, revision int64, actor uint64, action string, payload any) {
	if revision <= 0 {
		revision = time.Now().UnixNano()
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) == 0 {
		return
	}
	if err := s.repo.CreateSnapshot(context.WithoutCancel(ctx), &model.ContentSnapshot{
		ProjectID: projectID, EntityType: entityType, EntityID: entityID,
		Revision: revision, ActorID: actor, Action: action, Payload: string(data),
	}); err != nil {
		// 快照是审计增强，失败不回滚已成功的业务保存，但必须留痕便于排查。
		log.Printf("content snapshot failed: entity=%s id=%d err=%v", entityType, entityID, err)
	}
}

// ListRevisions 返回接口/文档的最近历史版本（不含 payload）。
func (s *Service) ListRevisions(ctx context.Context, userID uint64, userType int8, entityType string, id uint64) ([]dto.RevisionItem, error) {
	if err := s.checkRevisionEntity(ctx, userID, userType, entityType, id, model.PermQuery); err != nil {
		return nil, err
	}
	items, err := s.repo.ListSnapshots(ctx, entityType, id, revisionListLimit)
	if err != nil {
		return nil, err
	}
	out := make([]dto.RevisionItem, 0, len(items))
	for i := range items {
		out = append(out, snapshotItem(&items[i]))
	}
	return out, nil
}

// RevisionDiff 比较所选历史快照与当前内容，返回变化字段。
func (s *Service) RevisionDiff(ctx context.Context, userID uint64, userType int8, entityType string, id, revisionID uint64) (*dto.RevisionDiff, error) {
	current, err := s.revisionEntity(ctx, userID, userType, entityType, id, model.PermQuery)
	if err != nil {
		return nil, err
	}
	snap, err := s.revisionSnapshot(ctx, entityType, id, revisionID, current.projectID())
	if err != nil {
		return nil, err
	}
	before, err := payloadMap(snap.Payload)
	if err != nil {
		return nil, fmt.Errorf("解析历史快照: %w", err)
	}
	raw, err := json.Marshal(current.raw())
	if err != nil {
		return nil, fmt.Errorf("序列化当前内容: %w", err)
	}
	after, err := payloadMap(string(raw))
	if err != nil {
		return nil, fmt.Errorf("解析当前内容: %w", err)
	}
	return &dto.RevisionDiff{
		Revision: snapshotItem(snap),
		Fields:   diffSnapshot(entityType, before, after),
	}, nil
}

// RestoreRevision 把接口/文档回滚到指定历史快照；会再记一次变更与快照。
func (s *Service) RestoreRevision(ctx context.Context, userID uint64, userType int8, entityType string, id, revisionID uint64) error {
	current, err := s.revisionEntity(ctx, userID, userType, entityType, id, model.PermUpdate)
	if err != nil {
		return err
	}
	snap, err := s.revisionSnapshot(ctx, entityType, id, revisionID, current.projectID())
	if err != nil {
		return err
	}
	switch entityType {
	case model.SnapshotEntityAPI:
		return s.restoreApiSnapshot(ctx, userID, current.api, snap)
	case model.SnapshotEntityDoc:
		return s.restoreDocSnapshot(ctx, userID, current.doc, snap)
	default:
		return newBiz(errcode.CodeParamError, "不支持的快照类型")
	}
}

// revisionRef 统一承载 API/Doc 的当前实体（避免上层重复类型断言）。
type revisionRef struct {
	api *model.Api
	doc *model.Doc
}

func (r revisionRef) projectID() uint64 {
	if r.api != nil {
		return r.api.ProjectID
	}
	return r.doc.ProjectID
}

func (r revisionRef) raw() any {
	if r.api != nil {
		return r.api
	}
	return r.doc
}

// revisionEntity 取当前接口/文档并校验项目权限（query 或 update）。
func (s *Service) revisionEntity(ctx context.Context, userID uint64, userType int8, entityType string, id uint64, perm int8) (revisionRef, error) {
	switch entityType {
	case model.SnapshotEntityAPI:
		a, err := s.repo.GetApi(ctx, id)
		if err != nil {
			return revisionRef{}, err
		}
		if err := s.ensureProjectPermission(ctx, userID, userType, a.ProjectID, perm); err != nil {
			return revisionRef{}, err
		}
		return revisionRef{api: a}, nil
	case model.SnapshotEntityDoc:
		d, err := s.repo.GetDoc(ctx, id)
		if err != nil {
			return revisionRef{}, err
		}
		if err := s.ensureProjectPermission(ctx, userID, userType, d.ProjectID, perm); err != nil {
			return revisionRef{}, err
		}
		return revisionRef{doc: d}, nil
	default:
		return revisionRef{}, newBiz(errcode.CodeParamError, "类型必须是 api 或 doc")
	}
}

// checkRevisionEntity 只校验权限，不关心当前内容。
func (s *Service) checkRevisionEntity(ctx context.Context, userID uint64, userType int8, entityType string, id uint64, perm int8) error {
	_, err := s.revisionEntity(ctx, userID, userType, entityType, id, perm)
	return err
}

// revisionSnapshot 取快照并校验它确实属于该实体/项目。
func (s *Service) revisionSnapshot(ctx context.Context, entityType string, entityID, revisionID, projectID uint64) (*model.ContentSnapshot, error) {
	snap, err := s.repo.SnapshotByID(ctx, revisionID)
	if err != nil {
		return nil, err
	}
	if snap.EntityType != entityType || snap.EntityID != entityID || snap.ProjectID != projectID {
		return nil, newBiz(errcode.CodeParamError, "历史快照不属于当前条目")
	}
	return snap, nil
}

// restoreApiSnapshot 用历史载荷覆盖接口；保留 ID/项目/UID/版本链，避免丢关联。
func (s *Service) restoreApiSnapshot(ctx context.Context, userID uint64, cur *model.Api, snap *model.ContentSnapshot) error {
	var restored model.Api
	if err := json.Unmarshal([]byte(snap.Payload), &restored); err != nil {
		return newBiz(errcode.CodeFailed, "历史快照无法解析")
	}
	restored.ID = cur.ID
	restored.ProjectID = cur.ProjectID
	restored.ExtUID = ensureExtUID(cur.ExtUID)
	restored.LastChangeID = cur.LastChangeID
	restored.IsDeleted = 0
	restored.CreateTime = cur.CreateTime
	if err := s.repo.UpdateApi(ctx, &restored); err != nil {
		return err
	}
	cid := s.recordChange(model.SyncTypeAPI, restored.ExtUID, restored.ProjectID, restored.ID, model.SyncOpUpsert, userID)
	s.saveSnapshot(ctx, restored.ProjectID, model.SnapshotEntityAPI, restored.ID, cid, userID, model.SnapshotActionRestore, &restored)
	return nil
}

// restoreDocSnapshot 用历史载荷覆盖文档；保留 ID/项目/作者/UID，避免丢关联。
func (s *Service) restoreDocSnapshot(ctx context.Context, userID uint64, cur *model.Doc, snap *model.ContentSnapshot) error {
	var restored model.Doc
	if err := json.Unmarshal([]byte(snap.Payload), &restored); err != nil {
		return newBiz(errcode.CodeFailed, "历史快照无法解析")
	}
	restored.ID = cur.ID
	restored.ProjectID = cur.ProjectID
	restored.UserID = cur.UserID
	restored.ExtUID = ensureExtUID(cur.ExtUID)
	restored.LastChangeID = cur.LastChangeID
	restored.IsDeleted = 0
	restored.CreateTime = cur.CreateTime
	if err := s.repo.UpdateDoc(ctx, &restored); err != nil {
		return err
	}
	cid := s.recordChange(model.SyncTypeDoc, restored.ExtUID, restored.ProjectID, restored.ID, model.SyncOpUpsert, userID)
	s.saveSnapshot(ctx, restored.ProjectID, model.SnapshotEntityDoc, restored.ID, cid, userID, model.SnapshotActionRestore, &restored)
	return nil
}

// snapshotItem 快照 → 列表项。
func snapshotItem(s *model.ContentSnapshot) dto.RevisionItem {
	return dto.RevisionItem{ID: s.ID, Revision: s.Revision, Action: s.Action, ActorID: s.ActorID, CreateTime: s.CreateTime}
}

// payloadMap 把 JSON 载荷转为字段 map。
func payloadMap(payload string) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		return nil, err
	}
	return out, nil
}

type revisionField struct {
	key   string
	label string
}

var apiRevisionFields = []revisionField{
	{"api_name", "接口名称"}, {"url", "请求 URL"}, {"http_method_type", "请求方法"},
	{"protocol_type", "请求协议"}, {"description", "说明"}, {"develop_language", "开发语言"},
	{"http_return_type", "返回类型"}, {"status", "状态"}, {"version", "版本"},
	{"http_request_header", "请求头"}, {"http_request_params", "请求参数"},
	{"http_return_params", "返回参数"}, {"http_return_sample", "响应样例"},
}

var docRevisionFields = []revisionField{
	{"title", "文档标题"}, {"content", "正文"}, {"group_id", "所属分组"},
	{"is_top", "置顶"}, {"icon", "图标"},
}

// diffSnapshot 逐字段比较，返回变化项（只比较白名单字段，避免 id/update_time 噪音）。
func diffSnapshot(entityType string, before, after map[string]any) []dto.FieldDiff {
	fields := apiRevisionFields
	if entityType == model.SnapshotEntityDoc {
		fields = docRevisionFields
	}
	out := make([]dto.FieldDiff, 0)
	for _, f := range fields {
		b, a := before[f.key], after[f.key]
		if reflect.DeepEqual(b, a) {
			continue
		}
		out = append(out, dto.FieldDiff{Field: f.key, Label: f.label, Before: b, After: a})
	}
	return out
}
