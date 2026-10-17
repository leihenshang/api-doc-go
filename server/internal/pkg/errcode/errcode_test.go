package errcode

// P2-24：错误码是全局契约（前端按码分支），数值改动等于破坏兼容，这里把值钉住。

import "testing"

// TestCodeValuesStable 数值与前端约定一致，改动必须同时改前端。
func TestCodeValuesStable(t *testing.T) {
	want := map[string]int{
		"CodeSuccess":      200,
		"CodeParamError":   22,
		"CodeFailed":       14,
		"CodeUnauthorized": 34,
		"CodeForbidden":    403,
	}
	got := map[string]int{
		"CodeSuccess":      CodeSuccess,
		"CodeParamError":   CodeParamError,
		"CodeFailed":       CodeFailed,
		"CodeUnauthorized": CodeUnauthorized,
		"CodeForbidden":    CodeForbidden,
	}
	for name, v := range want {
		if got[name] != v {
			t.Fatalf("%s 应为 %d，got %d（改这个值等于破坏前端契约）", name, v, got[name])
		}
	}
}

func TestMessage(t *testing.T) {
	if Message(CodeSuccess) != "success" {
		t.Fatalf("成功文案异常: %q", Message(CodeSuccess))
	}
	// 每个已定义错误码都必须有非空文案（否则前端会显示空提示）
	for _, code := range []int{CodeParamError, CodeFailed, CodeUnauthorized, CodeForbidden} {
		if Message(code) == "" {
			t.Fatalf("错误码 %d 缺少默认文案", code)
		}
	}
	// 未登记的码兜底 "failed"，不能返回空串
	if got := Message(99999); got != "failed" {
		t.Fatalf("未登记码应兜底 failed，got %q", got)
	}
}
