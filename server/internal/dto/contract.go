package dto

import "encoding/json"

// ContractDiffReq 契约校验入参：只读，不落库。
type ContractDiffReq struct {
	ProjectID uint64          `json:"project_id" binding:"required"`
	OpenAPI   json.RawMessage `json:"openapi" binding:"required"`
}

// ContractEndpoint 一个「方法 + 路径」端点。
type ContractEndpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// ContractChange 单个参数/端点的变化。
type ContractChange struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Param    string `json:"param"`
	In       string `json:"in"`
	Kind     string `json:"kind"` // param_added / param_removed / required_changed / type_changed
	Before   string `json:"before"`
	After    string `json:"after"`
	Breaking bool   `json:"breaking"`
}

// ContractDiffResult 契约校验报告。
type ContractDiffResult struct {
	Added         []ContractEndpoint `json:"added"`
	Removed       []ContractEndpoint `json:"removed"`
	Changes       []ContractChange   `json:"changes"`
	BreakingCount int                `json:"breaking_count"`
}
