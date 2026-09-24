package page

// Query 分页入参。
type Query struct {
	PageNum  int `form:"page_num" json:"page_num"`
	PageSize int `form:"page_size" json:"page_size"`
}

// Normalize 归一分页参数：页码最小 1，每页 1~200，缺省 10。
func (q *Query) Normalize() {
	if q.PageNum < 1 {
		q.PageNum = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 10
	}
	if q.PageSize > 200 {
		q.PageSize = 200
	}
}

// Offset 返回 SQL OFFSET 值（需先 Normalize）。
func (q Query) Offset() int { return (q.PageNum - 1) * q.PageSize }

// Result 统一分页返回结构。
type Result[T any] struct {
	Items     []T   `json:"items"`
	Total     int64 `json:"total"`
	PageNum   int   `json:"pageNum"`
	PageSize  int   `json:"pageSize"`
	PageTotal int   `json:"pageTotal"`
}

// NewResult 依据总数与页参数组装分页结果。
func NewResult[T any](items []T, total int64, q Query) Result[T] {
	pageTotal := 0
	if q.PageSize > 0 {
		pageTotal = int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	}
	return Result[T]{Items: items, Total: total, PageNum: q.PageNum, PageSize: q.PageSize, PageTotal: pageTotal}
}
