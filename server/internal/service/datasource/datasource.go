// Package datasource 封装对外部数据库的只读元数据访问（introspect）。
// 驱动经空白导入注册；service 层仅依赖 database/sql，新增库类型时只改本包。
package datasource

import (
	"database/sql"
	"fmt"

	// 驱动注册：新增库类型时在此追加空白导入，service 层不感知。
	_ "github.com/go-sql-driver/mysql" // MySQL
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL
)

// 连接类型，与前端数据字典 type 选项及 model.DatabaseConn.Type 对齐。
const (
	TypeMySQL    int8 = 1
	TypePostgres int8 = 2
)

// ConnInfo 描述一次外部库连接。Password 为解密后的明文。
type ConnInfo struct {
	Type     int8
	Host     string
	Port     int
	User     string
	Password string
	Name     string // 数据库名
}

// Open 按类型建立连接池。驱动未注册或类型不支持时返回错误。
func Open(c ConnInfo) (*sql.DB, error) {
	driver, dsn, err := dsnFor(c)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("datasource: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("datasource: connect %s:%d: %w", c.Host, c.Port, err)
	}
	return db, nil
}

// dsnFor 返回驱动名与连接串。
func dsnFor(c ConnInfo) (driver, dsn string, err error) {
	switch c.Type {
	case TypePostgres:
		return "pgx", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=5",
			c.Host, c.Port, c.User, c.Password, c.Name), nil
	case TypeMySQL:
		return "mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&timeout=5s&readTimeout=10s",
			c.User, c.Password, c.Host, c.Port, c.Name), nil
	default:
		return "", "", fmt.Errorf("datasource: unsupported connection type %d", c.Type)
	}
}

// ListTables 返回库内基表（名称+注释）。表名仅允许标识符字符后拼接。
func ListTables(db *sql.DB, connType int8) ([]TableInfo, error) {
	var q string
	if connType == TypePostgres {
		q = `SELECT t.tablename, COALESCE(pgc.description, '')
			FROM pg_tables t
			JOIN pg_class c ON c.relname = t.tablename AND c.relkind = 'r'
			LEFT JOIN pg_description pgc ON pgc.objoid = c.oid AND pgc.objsubid = 0
			WHERE t.schemaname = 'public' ORDER BY t.tablename`
	} else {
		q = `SELECT table_name, table_comment FROM information_schema.tables
			WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' ORDER BY table_name`
	}
	return queryTables(db, q)
}

// ListColumns 返回指定表的列信息。table 参数先经标识符校验再使用。
func ListColumns(db *sql.DB, connType int8, table string) ([]ColumnInfo, error) {
	if err := checkIdent(table); err != nil {
		return nil, err
	}
	var q string
	if connType == TypePostgres {
		q = fmt.Sprintf(`SELECT c.column_name, c.data_type, COALESCE(d.description, '')
			FROM information_schema.columns c
			LEFT JOIN (
				SELECT a.attname AS col, pg_description.description
				FROM pg_description
				JOIN pg_class cl ON cl.oid = pg_description.objoid
				JOIN pg_attribute a ON a.attrelid = cl.oid AND a.attnum = pg_description.objsubid
				WHERE cl.relname = '%s'
			) d ON d.col = c.column_name
			WHERE c.table_name = '%s' ORDER BY c.ordinal_position`, table, table)
	} else {
		q = fmt.Sprintf(`SELECT column_name, data_type, column_comment
			FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = '%s' ORDER BY ordinal_position`, table)
	}
	rows, err := db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("datasource: columns of %s: %w", table, err)
	}
	defer rows.Close()
	var out []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		if err := rows.Scan(&c.Name, c.DataType, &c.Comment); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// TableInfo 表元数据。
type TableInfo struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

// ColumnInfo 列元数据。
type ColumnInfo struct {
	Name     string `json:"name"`
	DataType string `json:"data_type"`
	Comment  string `json:"comment"`
}

func queryTables(db *sql.DB, q string) ([]TableInfo, error) {
	rows, err := db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("datasource: tables: %w", err)
	}
	defer rows.Close()
	var out []TableInfo
	for rows.Next() {
		var t TableInfo
		if err := rows.Scan(&t.Name, &t.Comment); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// checkIdent 校验 SQL 标识符（表/列名），防止元数据查询拼接注入。
func checkIdent(s string) error {
	if s == "" || len(s) > 128 {
		return fmt.Errorf("datasource: invalid identifier")
	}
	for i, r := range s {
		ok := r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' && i > 0
		if !ok {
			return fmt.Errorf("datasource: invalid identifier %q", s)
		}
	}
	return nil
}
