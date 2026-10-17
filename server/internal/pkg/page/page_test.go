package page

// P2-24：page 是全局分页边界（页面大小上限、offset 计算、总页数），
// 边界算错会直接变成「翻到第 N 页拿到重复/缺失数据」，但此前零测试。

import "testing"

func TestNormalizeBounds(t *testing.T) {
	cases := []struct {
		in                Query
		wantNum, wantSize int
	}{
		{Query{}, 1, 10}, // 全零 → 默认
		{Query{PageNum: -3, PageSize: -1}, 1, 10},  // 负数 → 默认
		{Query{PageNum: 2, PageSize: 20}, 2, 20},   // 合法值原样
		{Query{PageNum: 1, PageSize: 200}, 1, 200}, // 上限边界
		{Query{PageNum: 1, PageSize: 201}, 1, 200}, // 超上限被夹住
		{Query{PageNum: 0, PageSize: 1000}, 1, 200},
	}
	for _, c := range cases {
		q := c.in
		q.Normalize()
		if q.PageNum != c.wantNum || q.PageSize != c.wantSize {
			t.Fatalf("%+v 归一化后 = (%d,%d)，期望 (%d,%d)", c.in, q.PageNum, q.PageSize, c.wantNum, c.wantSize)
		}
	}
}

func TestOffset(t *testing.T) {
	cases := []struct {
		q    Query
		want int
	}{
		{Query{PageNum: 1, PageSize: 10}, 0},
		{Query{PageNum: 2, PageSize: 10}, 10},
		{Query{PageNum: 5, PageSize: 200}, 800},
	}
	for _, c := range cases {
		if got := c.q.Offset(); got != c.want {
			t.Fatalf("Offset(%+v) = %d，期望 %d", c.q, got, c.want)
		}
	}
}

func TestNewResultPageTotal(t *testing.T) {
	cases := []struct {
		total, size, wantTotal int
	}{
		{0, 10, 0},
		{1, 10, 1},
		{10, 10, 1},
		{11, 10, 2}, // 整除后可多一页
		{200, 200, 1},
		{201, 200, 2},
	}
	for _, c := range cases {
		q := Query{PageNum: 1, PageSize: c.size}
		res := NewResult([]string{"x"}, int64(c.total), q)
		if res.PageTotal != c.wantTotal {
			t.Fatalf("total=%d size=%d 页数应为 %d，got %d", c.total, c.size, c.wantTotal, res.PageTotal)
		}
		if res.Total != int64(c.total) || res.PageNum != 1 || res.PageSize != c.size {
			t.Fatalf("结果字段异常: %+v", res)
		}
	}
	// PageSize 为 0 时不能除零
	if got := NewResult([]string{}, 5, Query{}); got.PageTotal != 0 {
		t.Fatalf("PageSize=0 时页数应为 0（避免除零），got %d", got.PageTotal)
	}
}
