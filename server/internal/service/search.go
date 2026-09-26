// 全局搜索：跨项目检索 项目 / 接口 / 项目文档（文档含正文）。
// 可见性：管理员搜全量，其它用户只搜自己参与的项目（见 searchableProjectIDs）。
package service

import (
	"context"
	"sort"
	"strings"
	"time"
)

// SearchItem 搜索命中项（前端直接渲染与跳转）。
type SearchItem struct {
	Kind        string    `json:"kind"`
	ID          uint64    `json:"id"`
	ProjectID   uint64    `json:"project_id"`
	ProjectName string    `json:"project_name"`
	Title       string    `json:"title"`
	Subtitle    string    `json:"subtitle"`
	Method      string    `json:"method"`
	UpdateTime  time.Time `json:"update_time"`
}

// SearchResult 按类型分组的搜索结果（各组已按相关度/更新时间排序并截断）。
type SearchResult struct {
	Keyword  string       `json:"keyword"`
	Projects []SearchItem `json:"projects"`
	Apis     []SearchItem `json:"apis"`
	Docs     []SearchItem `json:"docs"`
}

// 单组返回上限与请求上限。
const (
	searchDefaultLimit = 10
	searchMaxLimit     = 50
)

// Search 全局搜索；keyword 为空时返回空结果（不报错）。
func (s *Service) Search(ctx context.Context, userID uint64, typ int8, keyword string, limit int) (*SearchResult, error) {
	kw := strings.TrimSpace(keyword)
	res := &SearchResult{Keyword: kw, Projects: []SearchItem{}, Apis: []SearchItem{}, Docs: []SearchItem{}}
	if kw == "" {
		return res, nil
	}
	if limit <= 0 {
		limit = searchDefaultLimit
	}
	if limit > searchMaxLimit {
		limit = searchMaxLimit
	}

	ids, err := s.searchableProjectIDs(ctx, userID, typ)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return res, nil
	}
	projects, err := s.repo.ProjectsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	nameByID := make(map[uint64]string, len(projects))
	for _, p := range projects {
		nameByID[p.ID] = p.Title
	}

	if hits, err := s.repo.SearchProjects(ctx, ids, kw, limit); err != nil {
		return nil, err
	} else {
		for _, p := range hits {
			res.Projects = append(res.Projects, SearchItem{
				Kind: "project", ID: p.ID, ProjectID: p.ID, ProjectName: p.Title,
				Title: p.Title, Subtitle: trimSummary(p.Description), UpdateTime: p.UpdateTime,
			})
		}
	}

	apis, err := s.repo.SearchApis(ctx, ids, kw, limit)
	if err != nil {
		return nil, err
	}
	// 名称命中优先，其余保持「最近更新在前」。
	sort.SliceStable(apis, func(i, j int) bool {
		return containsFold(apis[i].ApiName, kw) && !containsFold(apis[j].ApiName, kw)
	})
	for i, a := range apis {
		if i >= limit {
			break
		}
		res.Apis = append(res.Apis, SearchItem{
			Kind: "api", ID: a.ID, ProjectID: a.ProjectID, ProjectName: nameByID[a.ProjectID],
			Title: a.ApiName, Subtitle: strings.TrimSpace(a.URL),
			Method: strings.ToUpper(strings.TrimSpace(a.HttpMethodType)), UpdateTime: a.UpdateTime,
		})
	}

	docs, err := s.repo.SearchDocs(ctx, ids, kw, limit)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(docs, func(i, j int) bool {
		return containsFold(docs[i].Title, kw) && !containsFold(docs[j].Title, kw)
	})
	for i, d := range docs {
		if i >= limit {
			break
		}
		res.Docs = append(res.Docs, SearchItem{
			Kind: "doc", ID: d.ID, ProjectID: d.ProjectID, ProjectName: nameByID[d.ProjectID],
			Title: d.Title, Subtitle: trimSummary(d.Content), UpdateTime: d.UpdateTime,
		})
	}
	return res, nil
}

// searchableProjectIDs 计算可搜索的项目范围：管理员为全量，其余为参与的项目。
func (s *Service) searchableProjectIDs(ctx context.Context, userID uint64, typ int8) ([]uint64, error) {
	if isAdmin(typ) {
		return s.repo.AllProjectIDs(ctx)
	}
	return s.repo.MemberProjectIDs(ctx, userID)
}

// containsFold 大小写不敏感包含判断（用于相关度排序）。
func containsFold(text, kw string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(kw))
}

// trimSummary 去 Markdown 记号后压空白并截断，作为列表摘要。
func trimSummary(content string) string {
	repl := strings.NewReplacer("#", "", "*", "", "`", "", ">", "", "|", " ", "\r", "", "\n", " ", "\t", " ")
	text := strings.Join(strings.Fields(repl.Replace(content)), " ")
	runes := []rune(text)
	if len(runes) > 60 {
		return string(runes[:60]) + "…"
	}
	return text
}
