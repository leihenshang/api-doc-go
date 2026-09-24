// Package errcode 定义统一的业务错误码。
// 响应体恒为 {code,msg,data}，HTTP 状态码恒为 200（沿用旧约定）。
package errcode

// 业务错误码。数值与前端约定保持一致，勿随意变更。
const (
	// CodeSuccess 表示成功。
	CodeSuccess = 200
	// CodeParamError 表示参数错误。
	CodeParamError = 22
	// CodeFailed 表示通用业务失败。
	CodeFailed = 14
	// CodeUnauthorized 表示未登录或登录过期，前端据此跳转登录页。
	CodeUnauthorized = 34
	// CodeForbidden 表示无操作权限。
	CodeForbidden = 403
)

// msgByCode 是错误码到默认提示文案的映射。
var msgByCode = map[int]string{
	CodeSuccess:      "success",
	CodeParamError:   "参数错误",
	CodeFailed:       "failed",
	CodeUnauthorized: "没有找到用户信息或登陆超时，请重新登陆试试",
	CodeForbidden:    "没有操作权限",
}

// Message 返回错误码对应的默认文案；未登记时返回 "failed"。
func Message(code int) string {
	if msg, ok := msgByCode[code]; ok {
		return msg
	}
	return "failed"
}
