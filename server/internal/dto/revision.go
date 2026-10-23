package dto

import "time"

// RevisionItem 历史版本列表项（不携带完整 payload）。
type RevisionItem struct {
	ID         uint64    `json:"id"`
	Revision   int64     `json:"revision"`
	Action     string    `json:"action"`
	ActorID    uint64    `json:"actor_id"`
	CreateTime time.Time `json:"create_time"`
}

// FieldDiff 单字段变化：Before 为所选历史快照，After 为当前内容。
type FieldDiff struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// RevisionDiff 历史快照与当前内容的字段级差异。
type RevisionDiff struct {
	Revision RevisionItem `json:"revision"`
	Fields   []FieldDiff  `json:"fields"`
}

// RevisionRestoreReq 回滚到指定历史快照。
type RevisionRestoreReq struct {
	Type       string `json:"type" binding:"required"` // api|doc
	ID         uint64 `json:"id" binding:"required"`
	RevisionID uint64 `json:"revision_id" binding:"required"`
}
