package service

import (
	"context"
	"database/sql"
	"encoding/json"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/crypto"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/service/datasource"
)

// ---- 外部数据库连接（数据字典） ----

// CreateConn 新建外部库连接；口令以 AES-256-GCM 加密后存储。
func (s *Service) CreateConn(ctx context.Context, userID uint64, typ int8, req dto.DatabaseReq) (*model.DatabaseConn, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermCreate); err != nil {
		return nil, err
	}
	if req.Password == "" {
		return nil, newBiz(errcode.CodeParamError, "连接口令不能为空")
	}
	pwd, err := crypto.Encrypt(req.Password, s.cfg.Encrypt.SecretKey)
	if err != nil {
		return nil, err
	}
	v := &model.DatabaseConn{
		ProjectID: req.ProjectID, UserID: userID, Address: req.Address, Port: req.Port,
		Username: req.Username, Password: pwd, DatabaseName: req.DatabaseName,
		Type: req.Type, Description: req.Description,
	}
	if v.Type == 0 {
		v.Type = datasource.TypeMySQL
	}
	if err := s.repo.CreateDatabaseConn(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// ListConns 列出项目的外部库连接。
func (s *Service) ListConns(ctx context.Context, projectID uint64) ([]model.DatabaseConn, error) {
	return s.repo.ListDatabaseConns(ctx, projectID)
}

// UpdateConn 更新连接。
func (s *Service) UpdateConn(ctx context.Context, userID uint64, typ int8, req dto.DatabaseReq) (*model.DatabaseConn, error) {
	v, err := s.repo.GetDatabaseConn(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, v.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	v.Address, v.Port, v.Username = req.Address, req.Port, req.Username
	if req.Password != "" {
		pwd, err := crypto.Encrypt(req.Password, s.cfg.Encrypt.SecretKey)
		if err != nil {
			return nil, err
		}
		v.Password = pwd
	}
	v.DatabaseName, v.Type, v.Description = req.DatabaseName, req.Type, req.Description
	if err := s.repo.UpdateDatabaseConn(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// DeleteConn 删除连接。
func (s *Service) DeleteConn(ctx context.Context, userID uint64, typ int8, id uint64) error {
	v, err := s.repo.GetDatabaseConn(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, v.ProjectID, model.PermDelete); err != nil {
		return err
	}
	return s.repo.DeleteDatabaseConn(ctx, id)
}

// ConnTables 读取外部库连接的表清单（只读元数据查询）。
func (s *Service) ConnTables(ctx context.Context, userID uint64, typ int8, connID uint64) ([]datasource.TableInfo, error) {
	db, conn, err := s.openExternal(ctx, userID, typ, connID)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tables, err := datasource.ListTables(db, conn.Type)
	if err != nil {
		return nil, newBiz(errcode.CodeFailed, "%v", err)
	}
	return tables, nil
}

// ConnColumns 读取外部库连接指定表的列信息。
func (s *Service) ConnColumns(ctx context.Context, userID uint64, typ int8, connID uint64, table string) ([]datasource.ColumnInfo, error) {
	db, conn, err := s.openExternal(ctx, userID, typ, connID)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	cols, err := datasource.ListColumns(db, conn.Type, table)
	if err != nil {
		return nil, newBiz(errcode.CodeFailed, "%v", err)
	}
	return cols, nil
}

// openExternal 校验查询权限后解密口令并建立外部库连接。
func (s *Service) openExternal(ctx context.Context, userID uint64, typ int8, connID uint64) (*sql.DB, *model.DatabaseConn, error) {
	conn, err := s.repo.GetDatabaseConn(ctx, connID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.ensureProjectPermission(ctx, userID, typ, conn.ProjectID, model.PermQuery); err != nil {
		return nil, nil, err
	}
	pwd, err := crypto.Decrypt(conn.Password, s.cfg.Encrypt.SecretKey)
	if err != nil {
		return nil, nil, newBiz(errcode.CodeFailed, "口令解密失败，请重新保存连接")
	}
	db, err := datasource.Open(datasource.ConnInfo{
		Type: conn.Type, Host: conn.Address, Port: conn.Port,
		User: conn.Username, Password: pwd, Name: conn.DatabaseName,
	})
	if err != nil {
		return nil, nil, newBiz(errcode.CodeFailed, "%v", err)
	}
	return db, conn, nil
}

// SaveFile 记录已上传文件。
func (s *Service) SaveFile(ctx context.Context, title, path string) (*model.FileRecord, error) {
	f := &model.FileRecord{Title: title, Path: path}
	if err := s.repo.CreateFile(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// ---- OpenAPI 导入 ----

// openAPI 是标准 OpenAPI 3.x 文档的最小解析模型（JSON 形式）。
type openAPI struct {
	Info struct {
		Title string `json:"title"`
	} `json:"info"`
	Paths map[string]map[string]struct {
		Summary     string   `json:"summary"`
		OperationID string   `json:"operationId"`
		Tags        []string `json:"tags"`
	} `json:"paths"`
}

// ImportOpenAPI 解析标准 OpenAPI(JSON) 并转为接口记录落库到指定项目+分组。
// 返回导入条数。每个 (path, method) 生成一条接口记录；tag 映射为二级分组。
func (s *Service) ImportOpenAPI(ctx context.Context, userID uint64, typ int8, projectID, groupID uint64, body []byte) (int, error) {
	if err := s.ensureProjectPermission(ctx, userID, typ, projectID, model.PermCreate); err != nil {
		return 0, err
	}
	var doc openAPI
	if err := json.Unmarshal(body, &doc); err != nil {
		return 0, newBiz(errcode.CodeParamError, "OpenAPI 解析失败：%v", err)
	}
	groups, err := s.repo.ListGroups(ctx, projectID, model.GroupTypeAPI)
	if err != nil {
		return 0, err
	}
	count := 0
	for path, ops := range doc.Paths {
		for method, op := range ops {
			target := s.resolveGroup(ctx, projectID, groupID, op.Tags, groups)
			a := &model.Api{
				ProjectID: projectID, GroupID: target,
				ApiName:        firstNonEmpty(op.Summary, op.OperationID, method+" "+path),
				HttpMethodType: upperMethod(method), URL: path, ProtocolType: "http",
			}
			if err := s.repo.CreateApi(ctx, a); err != nil {
				return count, err
			}
			count++
		}
	}
	s.recordOp(ctx, projectID, userID, projectID, model.LogObjectAPI, "import", "导入 OpenAPI 接口 "+itoa(count)+" 条")
	return count, nil
}

// resolveGroup 依据 tag 找到或创建二级分组，返回其 id。
func (s *Service) resolveGroup(ctx context.Context, projectID, defaultGroup uint64, tags []string, existing []model.Group) uint64 {
	if len(tags) == 0 {
		return defaultGroup
	}
	tag := tags[0]
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
