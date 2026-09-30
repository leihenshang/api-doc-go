package service

import (
	"context"
	"strings"

	shopenapi "github.com/zqstudio/api-doc-go-share/openapi"

	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// SaveFile 记录已上传文件。
func (s *Service) SaveFile(ctx context.Context, title, path string) (*model.FileRecord, error) {
	f := &model.FileRecord{Title: title, Path: path}
	if err := s.repo.CreateFile(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// SaveFileWithHash 内容寻址保存（S13）：hash 为 sha256 hex。
func (s *Service) SaveFileWithHash(ctx context.Context, title, path, hash, mime string, size int64) (*model.FileRecord, error) {
	f := &model.FileRecord{Title: title, Path: path, Hash: hash, Mime: mime, Size: size}
	if err := s.repo.CreateFile(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// GetFileByHash 按内容 hash 取文件记录（离线文档图片映射，R9）。
func (s *Service) GetFileByHash(ctx context.Context, hash string) (*model.FileRecord, error) {
	return s.repo.GetFileByHash(ctx, hash)
}

// ---- 接口导入（OpenAPI / Bruno） ----

// 导入来源标记，写入 api.source；空表示手工创建。
const (
	SourceOpenAPI = "openapi-import"
	SourceBruno   = "bruno-import"
	SourcePostman = "postman-import"
)

// 导入策略：覆盖=清空项目现有接口与接口目录后整体导入；追加=保留现状只新增（同方法+路径跳过）。
const (
	StrategyOverwrite = "overwrite"
	StrategyAppend    = "append"
)

// ImportResult 导入结果：imported 新增条数、skipped 因已存在跳过条数、failed 解析失败条数。
// Failures 给出失败明细（最多 10 条，供前端展示「状态说明」）。
type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Failed   int      `json:"failed"`
	Failures []string `json:"failures,omitempty"`
}

// addFailure 记录一条失败明细（明细最多保留 10 条，避免响应过大）。
func (r *ImportResult) addFailure(msg string) {
	r.Failed++
	if len(r.Failures) < 10 {
		r.Failures = append(r.Failures, msg)
	}
}

// normalizeStrategy 归一化导入策略，空值与未知值按「只新增」处理。
func normalizeStrategy(s string) string {
	if s == StrategyOverwrite {
		return StrategyOverwrite
	}
	return StrategyAppend
}

// apiUniqueKey 接口的去重键：大写方法 + 路径。
func apiUniqueKey(method, url string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(url)
}

// prepareImport 按策略准备项目：覆盖时清空现有接口与接口目录（结果集为纯导入内容），
// 追加时返回现存接口去重键集合（同键跳过）。
func (s *Service) prepareImport(ctx context.Context, projectID uint64, strategy string) (map[string]struct{}, error) {
	apis, err := s.repo.ListApisByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if normalizeStrategy(strategy) == StrategyOverwrite {
		if err := s.repo.SoftDeleteApisByProject(ctx, projectID); err != nil {
			return nil, err
		}
		if err := s.repo.SoftDeleteGroupsByProject(ctx, projectID, model.GroupTypeAPI); err != nil {
			return nil, err
		}
		return map[string]struct{}{}, nil
	}
	keys := make(map[string]struct{}, len(apis))
	for _, a := range apis {
		keys[apiUniqueKey(a.HttpMethodType, a.URL)] = struct{}{}
	}
	return keys, nil
}

// ImportOpenAPI 解析标准 OpenAPI(JSON) 并转为接口记录落库到指定项目+分组。
// 每个 (path, method) 生成一条接口记录；tag 映射为二级分组；写入来源 openapi-import。
func (s *Service) ImportOpenAPI(ctx context.Context, userID uint64, typ int8, projectID, groupID uint64, strategy string, body []byte) (ImportResult, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermCreate); err != nil {
		return ImportResult{}, err
	}
	keys, err := s.prepareImport(ctx, projectID, strategy)
	if err != nil {
		return ImportResult{}, err
	}
	doc, err := shopenapi.Decode(body)
	if err != nil {
		return ImportResult{}, newBiz(errcode.CodeParamError, "OpenAPI 解析失败：%v", err)
	}
	groups, err := s.repo.ListGroups(ctx, projectID, model.GroupTypeAPI)
	if err != nil {
		return ImportResult{}, err
	}
	var res ImportResult
	for _, op := range doc.Ops {
		method := upperMethod(op.Method)
		if _, dup := keys[apiUniqueKey(method, op.Path)]; dup {
			res.Skipped++
			continue
		}
		target := s.resolveGroup(ctx, projectID, groupID, op.Tags, groups)
		a := &model.Api{
			ProjectID: projectID, GroupID: target,
			ApiName:        firstNonEmpty(op.Summary, op.OperationID, method+" "+op.Path),
			HttpMethodType: method, URL: op.Path, ProtocolType: "http",
			Source: SourceOpenAPI,
		}
		if err := s.repo.CreateApi(ctx, a); err != nil {
			return res, err
		}
		keys[apiUniqueKey(method, op.Path)] = struct{}{}
		res.Imported++
	}
	s.recordOp(ctx, projectID, userID, projectID, model.LogObjectAPI, "import",
		"导入 OpenAPI 接口 "+itoa(res.Imported)+" 条（跳过 "+itoa(res.Skipped)+"）")
	return res, nil
}

// resolveGroup 依据 tag 找到或创建二级分组，返回其 id；空白 tag 归入默认分组，避免生成无标题的空分组。
func (s *Service) resolveGroup(ctx context.Context, projectID, defaultGroup uint64, tags []string, existing []model.Group) uint64 {
	if len(tags) == 0 {
		return defaultGroup
	}
	tag := strings.TrimSpace(tags[0])
	if tag == "" {
		return defaultGroup
	}
	for _, g := range existing {
		if g.Title == tag {
			return g.ID
		}
	}
	g := &model.Group{ProjectID: projectID, PID: defaultGroup, Title: tag, Type: model.GroupTypeAPI}
	if err := s.repo.CreateGroup(ctx, g); err == nil {
		existing = append(existing, *g)
		return g.ID
	}
	return defaultGroup
}

// firstNonEmpty 返回首个非空字符串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// upperMethod 归一化 HTTP 方法为大写。
func upperMethod(m string) string {
	b := []byte(m)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}

// itoa 简易整数转字符串。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
