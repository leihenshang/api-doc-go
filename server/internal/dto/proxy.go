package dto

// ProxyKV 在线调试的请求头 / 查询参数 / 表单行。
// Enabled 缺省（nil）视为启用，与客户端集合的 enabled 语义一致。
type ProxyKV struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Enabled *bool  `json:"enabled"`
}

// ProxyAuth 在线调试的认证配置（与客户端集合 auth 的 JSON 形态对齐）。
type ProxyAuth struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
	Key      string `json:"key"`
	Value    string `json:"value"`
	In       string `json:"in"`
}

// ProxySendReq 在线调试请求：由接口详情页按当前表单构造，服务端负责变量渲染与代发。
type ProxySendReq struct {
	ProjectID uint64     `json:"project_id" binding:"required"`
	APIID     uint64     `json:"api_id"`
	EnvID     uint64     `json:"env_id"`
	Method    string     `json:"method" binding:"required"`
	URL       string     `json:"url" binding:"required"`
	Headers   []ProxyKV  `json:"headers"`
	Query     []ProxyKV  `json:"query"`
	BodyType  string     `json:"body_type"` // none / json / text / form
	Body      string     `json:"body"`
	Form      []ProxyKV  `json:"form"`
	Auth      *ProxyAuth `json:"auth"`
}

// ProxyHeader 响应头：保留重复项（如多个 Set-Cookie）。
type ProxyHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ProxySendResp 在线调试响应：只回传目标服务的结果，不回显渲染后的请求（避免泄出敏感值）。
type ProxySendResp struct {
	Status      int           `json:"status"`
	StatusText  string        `json:"status_text"`
	Headers     []ProxyHeader `json:"headers"`
	Body        string        `json:"body"`
	ContentType string        `json:"content_type"`
	DurationMS  int64         `json:"duration_ms"`
	Size        int64         `json:"size"`
	Truncated   bool          `json:"truncated"`
	FinalURL    string        `json:"final_url"`
}
