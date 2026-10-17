package repository

import (
	"context"
	"time"

	"apidoc/internal/model"
	"apidoc/internal/pkg/page"

	"gorm.io/gorm"
)

// ================= 分组 =================

func (r *Repository) CreateGroup(ctx context.Context, v *model.Group) error {
	return gmCreate(ctx, r.db, v)
}
func (r *Repository) GetGroup(ctx context.Context, id uint64) (*model.Group, error) {
	return gmGet[model.Group](ctx, r.db, id)
}
func (r *Repository) UpdateGroup(ctx context.Context, v *model.Group) error {
	return gmSave(ctx, r.db, v)
}
func (r *Repository) DeleteGroup(ctx context.Context, id uint64) error {
	return gmHardDelete[model.Group](ctx, r.db, id)
}

// UpdateGroupSort 更新分组的父级与同级顺序。
func (r *Repository) UpdateGroupSort(ctx context.Context, id, pid uint64, priority int) error {
	return gmUpdateSort[model.Group](ctx, r.db, id, pid, "p_id", priority)
}

// DocGroupCounts 各分组下的文档数 + 未分组文档数（B19 Web 展示用）。
// 返回 map[groupID]count；未分组计数放在返回值第二项（group_id = 0）。
func (r *Repository) DocGroupCounts(ctx context.Context, projectID uint64) (map[uint64]int64, int64, error) {
	type row struct {
		GroupID uint64
		N       int64
	}
	var rows []row
	if err := r.active(ctx).Model(&model.Doc{}).
		Select("group_id, COUNT(*) AS n").
		Where("project_id = ? AND is_deleted = 0", projectID).
		Group("group_id").Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	counts := make(map[uint64]int64, len(rows))
	var ungrouped int64
	for _, x := range rows {
		if x.GroupID == 0 {
			ungrouped = x.N
			continue
		}
		counts[x.GroupID] = x.N
	}
	return counts, ungrouped, nil
}

// ListGroups 返回某项目某类型的分组（按优先级、id 排序）。
func (r *Repository) ListGroups(ctx context.Context, projectID uint64, typ int8) ([]model.Group, error) {
	var items []model.Group
	db := r.active(ctx).Where("project_id = ?", projectID)
	if typ >= 0 {
		db = db.Where("type = ?", typ)
	}
	err := db.Order("priority ASC, id ASC").Find(&items).Error
	return items, err
}

// ================= 接口 =================

func (r *Repository) CreateApi(ctx context.Context, v *model.Api) error {
	return gmCreate(ctx, r.db, v)
}
func (r *Repository) GetApi(ctx context.Context, id uint64) (*model.Api, error) {
	return gmGet[model.Api](ctx, r.db, id)
}
func (r *Repository) UpdateApi(ctx context.Context, v *model.Api) error { return gmSave(ctx, r.db, v) }
func (r *Repository) DeleteApi(ctx context.Context, id uint64) error {
	return gmHardDelete[model.Api](ctx, r.db, id)
}

// ApisByIDsAny 取指定 id 的接口（**含已删除**，供批量操作用）。
func (r *Repository) ApisByIDsAny(ctx context.Context, ids []uint64) ([]model.Api, error) {
	var items []model.Api
	if len(ids) == 0 {
		return items, nil
	}
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error
	return items, err
}

// DocsByIDsAny 取指定 id 的文档（**含已删除**）。
func (r *Repository) DocsByIDsAny(ctx context.Context, ids []uint64) ([]model.Doc, error) {
	var items []model.Doc
	if len(ids) == 0 {
		return items, nil
	}
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error
	return items, err
}

// GroupsByProjectsAny 取项目下指定类型的分组（**含已软删除**），供恢复时回溯父级链。
func (r *Repository) GroupsByProjectsAny(ctx context.Context, projectIDs []uint64, typ int8) ([]model.Group, error) {
	var items []model.Group
	if len(projectIDs) == 0 {
		return items, nil
	}
	err := r.db.WithContext(ctx).Where("project_id IN ? AND type = ?", projectIDs, typ).Find(&items).Error
	return items, err
}

// UpdateApiSort 更新接口的所属分组与同级顺序。
func (r *Repository) UpdateApiSort(ctx context.Context, id, groupID uint64, priority int) error {
	return gmUpdateSort[model.Api](ctx, r.db, id, groupID, "group_id", priority)
}

// ListApis 分页查询接口。deleted=0 正常、1 回收站。
func (r *Repository) ListApis(ctx context.Context, projectID, groupID uint64, deleted int8, keyword string, q page.Query) ([]model.Api, int64, error) {
	db := r.db.WithContext(ctx).Where("project_id = ?", projectID).Where("is_deleted = ?", deleted)
	if groupID > 0 {
		db = db.Where("group_id = ?", groupID)
	}
	if keyword != "" {
		like := keyword + "%"
		db = db.Where("api_name LIKE ? OR url LIKE ?", like, like)
	}
	total, err := gmCount[model.Api](db)
	if err != nil {
		return nil, 0, err
	}
	var items []model.Api
	err = db.Order("priority ASC, id ASC").Limit(q.PageSize).Offset(q.Offset()).Find(&items).Error
	return items, total, err
}

// ListApisByProject 取项目下全部未删除接口（导入去重用）。
func (r *Repository) ListApisByProject(ctx context.Context, projectID uint64) ([]model.Api, error) {
	var items []model.Api
	err := r.db.WithContext(ctx).Where("project_id = ? AND is_deleted = ?", projectID, 0).Find(&items).Error
	return items, err
}

// ApisByIDs 取项目下指定 id 的未删除接口（批量变量徽标用，避免逐条查询）。
func (r *Repository) ApisByIDs(ctx context.Context, projectID uint64, ids []uint64) ([]model.Api, error) {
	var items []model.Api
	if len(ids) == 0 {
		return items, nil
	}
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND is_deleted = 0 AND id IN ?", projectID, ids).Find(&items).Error
	return items, err
}

// SoftDeleteApisByProject 软删除项目下全部接口（导入「完全覆盖」时清空现状）。
func (r *Repository) SoftDeleteApisByProject(ctx context.Context, projectID uint64) error {
	return r.db.WithContext(ctx).Model(&model.Api{}).
		Where("project_id = ? AND is_deleted = 0", projectID).Update("is_deleted", 1).Error
}

// SoftDeleteGroupsByProject 软删除项目下某类型的全部分组（导入「完全覆盖」时清空接口目录）。
func (r *Repository) SoftDeleteGroupsByProject(ctx context.Context, projectID uint64, typ int8) error {
	return r.db.WithContext(ctx).Model(&model.Group{}).
		Where("project_id = ? AND type = ? AND is_deleted = 0", projectID, typ).Update("is_deleted", 1).Error
}

// ================= 文档 =================

func (r *Repository) CreateDoc(ctx context.Context, v *model.Doc) error {
	return gmCreate(ctx, r.db, v)
}
func (r *Repository) GetDoc(ctx context.Context, id uint64) (*model.Doc, error) {
	return gmGet[model.Doc](ctx, r.db, id)
}
func (r *Repository) UpdateDoc(ctx context.Context, v *model.Doc) error { return gmSave(ctx, r.db, v) }
func (r *Repository) DeleteDoc(ctx context.Context, id uint64) error {
	return gmHardDelete[model.Doc](ctx, r.db, id)
}

// UpdateDocSort 更新文档的所属分组与同级顺序。
func (r *Repository) UpdateDocSort(ctx context.Context, id, groupID uint64, priority int) error {
	return gmUpdateSort[model.Doc](ctx, r.db, id, groupID, "group_id", priority)
}

// ListDocs 分页查询文档（置顶优先）。deleted=0 正常、1 回收站。
func (r *Repository) ListDocs(ctx context.Context, projectID, groupID uint64, keyword string, deleted int8, q page.Query) ([]model.Doc, int64, error) {
	db := r.db.WithContext(ctx).Where("project_id = ?", projectID).Where("is_deleted = ?", deleted)
	if groupID > 0 {
		db = db.Where("group_id = ?", groupID)
	}
	if keyword != "" {
		db = db.Where("title LIKE ?", keyword+"%")
	}
	total, err := gmCount[model.Doc](db)
	if err != nil {
		return nil, 0, err
	}
	var items []model.Doc
	err = db.Order("is_top DESC, priority ASC, id ASC").Limit(q.PageSize).Offset(q.Offset()).Find(&items).Error
	return items, total, err
}

// IncrDocView 浏览数 +1。
func (r *Repository) IncrDocView(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.Doc{}).Where("id = ?", id).UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

// IncrDocLike 点赞数 +1。
func (r *Repository) IncrDocLike(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.Doc{}).Where("id = ?", id).UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}

// ================= Bug =================

func (r *Repository) CreateBug(ctx context.Context, v *model.Bug) error {
	return gmCreate(ctx, r.db, v)
}
func (r *Repository) GetBug(ctx context.Context, id uint64) (*model.Bug, error) {
	return gmGet[model.Bug](ctx, r.db, id)
}
func (r *Repository) UpdateBug(ctx context.Context, v *model.Bug) error { return gmSave(ctx, r.db, v) }
func (r *Repository) DeleteBug(ctx context.Context, id uint64) error {
	return gmHardDelete[model.Bug](ctx, r.db, id)
}

// ListBugs 分页查询 Bug。
func (r *Repository) ListBugs(ctx context.Context, projectID uint64, status int8, q page.Query) ([]model.Bug, int64, error) {
	db := r.active(ctx).Where("project_id = ?", projectID)
	if status > 0 {
		db = db.Where("status = ?", status)
	}
	total, err := gmCount[model.Bug](db)
	if err != nil {
		return nil, 0, err
	}
	var items []model.Bug
	err = db.Order("is_top DESC, id DESC").Limit(q.PageSize).Offset(q.Offset()).Find(&items).Error
	return items, total, err
}

func (r *Repository) CreateBugAssign(ctx context.Context, v *model.BugAssign) error {
	return gmCreate(ctx, r.db, v)
}

// ListBugAssign 返回某 Bug 的指派记录。
func (r *Repository) ListBugAssign(ctx context.Context, bugID uint64) ([]model.BugAssign, error) {
	var items []model.BugAssign
	err := r.active(ctx).Where("bug_id = ?", bugID).Order("id ASC").Find(&items).Error
	return items, err
}

// ================= 字段映射 =================

func (r *Repository) CreateFieldMapping(ctx context.Context, v *model.FieldMapping) error {
	return gmCreate(ctx, r.db, v)
}
func (r *Repository) GetFieldMapping(ctx context.Context, id uint64) (*model.FieldMapping, error) {
	return gmGet[model.FieldMapping](ctx, r.db, id)
}
func (r *Repository) UpdateFieldMapping(ctx context.Context, v *model.FieldMapping) error {
	return gmSave(ctx, r.db, v)
}
func (r *Repository) DeleteFieldMapping(ctx context.Context, id uint64) error {
	return gmHardDelete[model.FieldMapping](ctx, r.db, id)
}
func (r *Repository) ListFieldMappings(ctx context.Context, projectID uint64) ([]model.FieldMapping, error) {
	var items []model.FieldMapping
	err := r.active(ctx).Where("project_id = ?", projectID).Order("create_time DESC").Find(&items).Error
	return items, err
}

// FieldMappingByField 按字段名查映射（用于自动回填）。
func (r *Repository) FieldMappingByField(ctx context.Context, projectID uint64, field string) (*model.FieldMapping, error) {
	var v model.FieldMapping
	err := r.active(ctx).Where("project_id = ? AND field = ?", projectID, field).First(&v).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &v, nil
}

// ================= 公共属性 =================

func (r *Repository) ListProperties(ctx context.Context) ([]model.Property, error) {
	var items []model.Property
	err := r.db.WithContext(ctx).Order("id ASC").Find(&items).Error
	return items, err
}
func (r *Repository) CreateProperty(ctx context.Context, v *model.Property) error {
	return r.db.WithContext(ctx).Create(v).Error
}
func (r *Repository) UpdateProperty(ctx context.Context, v *model.Property) error {
	return r.db.WithContext(ctx).Save(v).Error
}
func (r *Repository) DeleteProperty(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Property{}, id).Error
}

// ================= 消息 / 验证码 =================

func (r *Repository) CreateMessage(ctx context.Context, v *model.Message) error {
	return gmCreate(ctx, r.db, v)
}

// MarkMessageUsed 将验证码标记为已使用。
func (r *Repository) MarkMessageUsed(ctx context.Context, id uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.Message{}).Where("id = ?", id).
		Updates(map[string]any{"is_used": 1, "used_time": now}).Error
}

// ================= 站内通知 =================

func (r *Repository) CreateNotification(ctx context.Context, v *model.Notification) error {
	return gmCreate(ctx, r.db, v)
}
func (r *Repository) ListNotifications(ctx context.Context, receiver uint64, q page.Query) ([]model.Notification, int64, error) {
	db := r.active(ctx).Where("receiver = ?", receiver)
	total, err := gmCount[model.Notification](db)
	if err != nil {
		return nil, 0, err
	}
	var items []model.Notification
	err = db.Order("id DESC").Limit(q.PageSize).Offset(q.Offset()).Find(&items).Error
	return items, total, err
}

// MarkNotificationRead 标记通知已读：必须带接收者条件，否则任意登录用户可标记他人通知（IDOR）。
func (r *Repository) MarkNotificationRead(ctx context.Context, receiver, id uint64) error {
	res := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("id = ? AND receiver = ?", id, receiver).
		Update("is_read", 1)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ================= 文件 =================

func (r *Repository) CreateFile(ctx context.Context, v *model.FileRecord) error {
	return gmCreate(ctx, r.db, v)
}

// GetFileByHash 按内容 hash 取文件记录（S13 内容寻址）。
func (r *Repository) GetFileByHash(ctx context.Context, hash string) (*model.FileRecord, error) {
	var f model.FileRecord
	err := r.db.WithContext(ctx).Where("hash = ?", hash).First(&f).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &f, nil
}

// ================= 操作日志 =================

func (r *Repository) CreateLog(ctx context.Context, v *model.OperationLog) error {
	return gmCreate(ctx, r.db, v)
}

// ListLogs 分页查询操作日志。
func (r *Repository) ListLogs(ctx context.Context, projectID, objectID uint64, typ int8, q page.Query) ([]model.OperationLog, int64, error) {
	db := r.active(ctx).Where("project_id = ?", projectID)
	if objectID > 0 {
		db = db.Where("object_id = ?", objectID)
	}
	if typ > 0 {
		db = db.Where("type = ?", typ)
	}
	total, err := gmCount[model.OperationLog](db)
	if err != nil {
		return nil, 0, err
	}
	var items []model.OperationLog
	err = db.Order("id DESC").Limit(q.PageSize).Offset(q.Offset()).Find(&items).Error
	return items, total, err
}
