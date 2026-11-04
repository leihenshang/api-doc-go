package service

// U3：导入 api-doc-go 客户端的集合整包（.apidocgo.json）。
//
// 契约来源：客户端 `internal/collection/bundle.go`（CollectionBundle）。
// 结构在各仓各自定义（客户端内部模型不进共享库），因此**字段名与语义必须逐字对齐**；
// 改动任一侧都要跑两端往返单测（客户端 TestBundleRoundTrip / 服务端 TestImportBundle）。

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/repository"
)

// bundleFormat / bundleMaxVersion 与客户端 collection.BundleFormat / BundleVersion 对齐。
const (
	bundleFormat     = "api-doc-go/collection"
	bundleMaxVersion = 1
)

// bundlePayload 集合整包（只取导入需要解析的字段；其余键忽略）。
type bundlePayload struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	// Collection 集合元信息（名称用于「未指定目标分组」时建顶层分组，这里只做统计）。
	Collection struct {
		Name string `json:"name"`
		UID  string `json:"uid"`
	} `json:"collection"`
	Folders []struct {
		Path string `json:"path"`
		Name string `json:"name"`
		Seq  int    `json:"seq"`
	} `json:"folders"`
	Requests []bundleRequest `json:"requests"`
	Docs     []bundleDoc     `json:"docs"`
}

// bundleRequest 一条请求（字段名对齐客户端 collection.Request 的 JSON tag）。
type bundleRequest struct {
	UID     string `json:"uid"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Method  string `json:"method"`
	URL     string `json:"url"`
	Summary string `json:"summary"`
	Docs    string `json:"docs"`
	// 以下均按客户端原文直接落服务端 JSON/文本列（服务端不解释语义）。
	Params     json.RawMessage `json:"params"`
	Headers    json.RawMessage `json:"headers"`
	Body       bundleBody      `json:"body"`
	Auth       json.RawMessage `json:"auth"`
	Settings   json.RawMessage `json:"settings"`
	Script     json.RawMessage `json:"script"`
	Vars       json.RawMessage `json:"varsPreRequest"`
	Asserts    json.RawMessage `json:"asserts"`
	RespDoc    bundleRespDoc   `json:"respDoc"`
	Status     int             `json:"status"`
	Version    string          `json:"version"`
	Deprecated string          `json:"deprecatedAt"`
}

// bundleBody 请求体（客户端 share.Body 的 JSON 形态）。
type bundleBody struct {
	Type   string          `json:"type"`
	Raw    string          `json:"raw"`
	Data   string          `json:"data"`
	Form   json.RawMessage `json:"form"`
	Descs  json.RawMessage `json:"descs"`
	Fields json.RawMessage `json:"fields"`
}

// bundleRespDoc 响应侧文档（客户端 collection.ResponseDoc）。
type bundleRespDoc struct {
	Type        string          `json:"type"`
	Params      json.RawMessage `json:"params"`
	Sample      string          `json:"sample"`
	ContentType string          `json:"contentType"`
}

// bundleDoc 文档条目（客户端 collection.DocEntry）。
type bundleDoc struct {
	UID     string `json:"uid"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
	Icon    string `json:"icon"`
	Seq     int    `json:"seq"`
}

// ImportBundle 导入集合整包到指定项目（groupID = 未归类条目的落点）。
//
// 与 OpenAPI 导入的差异：这里带**目录结构与顺序**，以及请求的富字段（脚本/断言/认证/
// 请求设置/响应文档），所以按 path 建分组、按 seq 保顺序，字段原样落列。
func (s *Service) ImportBundle(ctx context.Context, userID uint64, typ int8, projectID, groupID uint64, strategy string, body []byte) (ImportResult, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermCreate); err != nil {
		return ImportResult{}, err
	}
	b, err := decodeBundle(body)
	if err != nil {
		return ImportResult{}, newBiz(errcode.CodeParamError, "%v", err)
	}

	var res ImportResult
	// 与导入同事务：任一硬错误回滚，不留半截数据。
	err = s.repo.Transaction(ctx, func(tx *repository.Repository) error {
		sc := importScope{ctx: ctx, repo: tx}
		keys, kerr := s.prepareImport(sc, projectID, strategy)
		if kerr != nil {
			return kerr
		}

		// ① 目录：按包内顺序建（子目录的父路径一定在前），path → 分组 id。
		//
		// 同名同父的分组复用已有记录：同一个包导入两次不应该长出两套目录。
		// （请求侧按「方法+URL」去重，两边口径一致。）
		dirID := make(map[string]uint64, len(b.Folders))
		existing, lerr := tx.ListGroups(ctx, projectID, model.GroupTypeAPI)
		if lerr != nil {
			return lerr
		}
		existKey := make(map[string]uint64, len(existing))
		for _, g := range existing {
			existKey[fmt.Sprintf("%d\x00%s", g.PID, g.Title)] = g.ID
		}
		for _, f := range b.Folders {
			dir := strings.Trim(strings.TrimSpace(f.Path), "/")
			if dir == "" {
				continue
			}
			pid := groupID
			if parent := path.Dir(dir); parent != "." && parent != "" {
				if id, ok := dirID[parent]; ok {
					pid = id
				}
			}
			title := strings.TrimSpace(f.Name)
			if title == "" {
				title = path.Base(dir)
			}
			if id, ok := existKey[fmt.Sprintf("%d\x00%s", pid, title)]; ok {
				dirID[dir] = id
				continue
			}
			g := &model.Group{
				ProjectID: projectID, PID: pid, Title: title,
				Type: model.GroupTypeAPI, Priority: f.Seq, ExtUID: ensureExtUID(""),
			}
			if err := tx.CreateGroup(ctx, g); err != nil {
				return err
			}
			dirID[dir] = g.ID
			existKey[fmt.Sprintf("%d\x00%s", pid, title)] = g.ID
			res.Imported++
			s.recordChange(model.SyncTypeGroup, g.ExtUID, projectID, g.ID, model.SyncOpUpsert, userID)
		}

		// ② 请求：同方法同地址视为已存在（与 OpenAPI 导入同一去重口径）。
		for i := range b.Requests {
			r := &b.Requests[i]
			method := upperMethod(strings.TrimSpace(r.Method))
			url := strings.TrimSpace(r.URL)
			if method == "" || url == "" {
				res.Skipped++
				continue
			}
			if _, dup := keys[apiUniqueKey(method, url)]; dup {
				res.Skipped++
				continue
			}
			a := &model.Api{
				ProjectID: projectID, GroupID: groupIDForRequest(r.Path, dirID, groupID),
				ApiName:        firstNonEmpty(r.Name, method+" "+url),
				HttpMethodType: method, URL: url, ProtocolType: protoOf(method),
				HttpRequestSummary: r.Summary, Description: r.Docs,
				ExtUID: ensureExtUID(""), Source: "apidocgo-import",
			}
			applyBundleRequest(a, r)
			if err := tx.CreateApi(ctx, a); err != nil {
				return err
			}
			keys[apiUniqueKey(method, url)] = struct{}{}
			res.Imported++
			// 导入的条目也要进变更流水，否则已绑定的客户端永远拉不到（同 R1 口径）。
			s.recordChange(model.SyncTypeAPI, a.ExtUID, projectID, a.ID, model.SyncOpUpsert, userID)
		}

		// ③ 文档：按 meta 路径落对应分组。
		for i := range b.Docs {
			d := &b.Docs[i]
			name := strings.TrimSpace(d.Name)
			if name == "" {
				name = "未命名文档"
			}
			doc := &model.Doc{
				ProjectID: projectID, GroupID: groupIDForRequest(d.Path, dirID, groupID),
				Title: name, Content: d.Content, Icon: d.Icon,
				Priority: d.Seq, ExtUID: ensureExtUID(""),
			}
			if err := tx.CreateDoc(ctx, doc); err != nil {
				return err
			}
			res.Imported++
			s.recordChange(model.SyncTypeDoc, doc.ExtUID, projectID, doc.ID, model.SyncOpUpsert, userID)
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	s.recordOp(ctx, projectID, userID, projectID, model.LogObjectAPI, "import",
		"导入 api-doc-go 集合包 "+itoa(res.Imported)+" 条（跳过 "+itoa(res.Skipped)+"）")
	return res, nil
}

// decodeBundle 解析并校验整包标识与版本。
func decodeBundle(body []byte) (*bundlePayload, error) {
	var b bundlePayload
	if err := json.Unmarshal(body, &b); err != nil {
		return nil, fmt.Errorf("集合包解析失败（应为 %s 的 JSON）：%w", bundleFormat, err)
	}
	if strings.TrimSpace(b.Format) != bundleFormat {
		return nil, fmt.Errorf("不是 api-doc-go 集合包：format = %q（期望 %q）", b.Format, bundleFormat)
	}
	ver := b.Version
	if ver <= 0 {
		ver = 1 // 早期手写文件可能缺 version
	}
	if ver > bundleMaxVersion {
		return nil, fmt.Errorf("集合包版本 %d 高于本服务端支持的 %d，请先升级服务端", ver, bundleMaxVersion)
	}
	return &b, nil
}

// groupIDForRequest 由文件路径（如 api/user/登录.yml）推出所属分组 id；未命中回落导入目标分组。
func groupIDForRequest(filePath string, dirID map[string]uint64, fallback uint64) uint64 {
	dir := path.Dir(strings.Trim(strings.TrimSpace(filePath), "/"))
	if dir == "." || dir == "" {
		return fallback
	}
	if id, ok := dirID[dir]; ok {
		return id
	}
	return fallback
}

// protoOf 由方法推协议类型（客户端 gRPC 请求的 method 固定为 GRPC）。
func protoOf(method string) string {
	if strings.EqualFold(method, "GRPC") {
		return "grpc"
	}
	return "http"
}

// applyBundleRequest 把客户端富字段原样落到服务端列（服务端不解释语义）。
func applyBundleRequest(a *model.Api, r *bundleRequest) {
	a.HttpRequestParams = rawOrEmpty(r.Params)
	a.HttpRequestHeader = rawOrEmpty(r.Headers)
	a.HttpRequestBodyType = r.Body.Type
	// json/text 体的原文：客户端 Data 优先（Bruno 风格），否则 Raw。
	a.HttpRequestBody = firstNonEmpty(r.Body.Data, r.Body.Raw)
	a.HttpRequestBodyForm = rawOrEmpty(r.Body.Form)
	a.HttpRequestBodyDescs = rawOrEmpty(r.Body.Descs)
	a.HttpRequestBodyFields = rawOrEmpty(r.Body.Fields)
	a.HttpAuth = rawOrEmpty(r.Auth)
	a.RequestSettings = rawOrEmpty(r.Settings)
	a.HttpScript = rawOrEmpty(r.Script)
	a.ScriptVars = rawOrEmpty(r.Vars)
	a.ScriptAsserts = rawOrEmpty(r.Asserts)
	a.HttpReturnType = r.RespDoc.Type
	a.HttpReturnParams = rawOrEmpty(r.RespDoc.Params)
	a.HttpReturnSample = r.RespDoc.Sample
	a.HttpReturnContentType = r.RespDoc.ContentType
	if r.Status != 0 {
		a.Status = int8(r.Status)
	}
	a.Version = strings.TrimSpace(r.Version)
}

// rawOrEmpty 空/`null` 的 JSON 片段统一成空串（服务端列语义：空串 = 未设置）。
func rawOrEmpty(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	return s
}
