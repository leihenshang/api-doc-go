package repository

import (
	"context"

	"apidoc/internal/model"
)

// extUIDByID 按主键反查 ext_uid（只取一列，不为一个字符串把整行读出来）。
//
// 查不到一律返回空串而不是 error：调用方都是「拼同步载荷时补一个引用」，
// 父分组/所属接口被彻底删除后引用为空是合法状态，不该让整次拉取失败。
// 与 gmGet 的区别是有意不做 is_deleted 过滤 —— 子条目的 parent_uid 必须
// 指向真实的父 uid（哪怕父在回收站里），否则客户端重建树时会断开。
func (r *Repository) extUIDByID(ctx context.Context, m any, id uint64) string {
	if id == 0 {
		return ""
	}
	var uid string
	if err := r.db.WithContext(ctx).Model(m).Where("id = ?", id).Limit(1).Pluck("ext_uid", &uid).Error; err != nil {
		return ""
	}
	return uid
}

// GroupExtUID 分组 id → ext_uid（同步载荷的 parent_uid / group_uid）。
func (r *Repository) GroupExtUID(ctx context.Context, id uint64) string {
	return r.extUIDByID(ctx, &model.Group{}, id)
}

// DocExtUID 文档 id → ext_uid。
func (r *Repository) DocExtUID(ctx context.Context, id uint64) string {
	return r.extUIDByID(ctx, &model.Doc{}, id)
}

// ApiExtUID 接口 id → ext_uid（响应示例的 api_uid）。
func (r *Repository) ApiExtUID(ctx context.Context, id uint64) string {
	return r.extUIDByID(ctx, &model.Api{}, id)
}

// VarEnvExtUID 变量环境 id → ext_uid（变量的 env_uid）。
func (r *Repository) VarEnvExtUID(ctx context.Context, id uint64) string {
	return r.extUIDByID(ctx, &model.VariableEnv{}, id)
}
