package database

// 版本化迁移执行器（审核 H-14）。
//
// 背景：migrations/ 下的 SQL 脚本此前从未被执行——生产只跑 GORM AutoMigrate，
// 脚本与模型长期漂移（例如 file.hash/size/mime 根本没有迁移）。
// 现在：
//   - 脚本用 embed.FS 打包进二进制，启动时按版本号顺序执行；
//   - schema_migrations 记录已应用版本，每个版本在单事务内「执行 SQL + 记账」；
//   - 已有库（AutoMigrate 建立、尚无 schema_migrations）首次接入时先补模型再整体基线化，
//     不会重跑 0001 的 CREATE TABLE；
//   - AutoMigrate 只在开发/测试与基线化时使用，生产入口 MigrateRelease 不再依赖它；
//   - 每个版本必须 up/down 成对，两个方言的版本号与简述必须一致。

import (
	"fmt"
	"io/fs"
	"log"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"apidoc/internal/model"
	"apidoc/migrations"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const schemaMigrationsTable = "schema_migrations"

// legacyProjectTitleIndex 是 project.title 的历史全局唯一索引（不区分软删除），
// 会让软删除后的项目名被永久占用，表现为「列表里没有同名项目却创建失败」。
const legacyProjectTitleIndex = "idx_project_title"

// legacyUserNameEmailIndex 是 user_info 的历史全局唯一索引（不区分软删除）。
const legacyUserNameEmailIndex = "uk_name_email"

// legacyUserNameEmailActive 是 0015 建立的复合局部唯一索引 (name, email)。
// 0019 会把它拆成 name/email 两个独立索引（审核 H-15）。
const legacyUserNameEmailActive = "uk_name_email_active"

// 0012 建立的响应示例索引名与 GORM 默认命名（idx_<表>_<列>）不一致，0018 统一。
const (
	legacySampleProjectIndex = "idx_api_return_sample_project"
	legacySampleAPIIndex     = "idx_api_return_sample_api"
)

// migrationFilePattern 匹配 0001_init.up.sql，捕获版本号、简述与方向。
var migrationFilePattern = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.(up|down)\.sql$`)

// migration 一个版本的双向脚本。
type migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

// schemaMigration 对应 schema_migrations 表。
type schemaMigration struct {
	Version   int       `gorm:"column:version;primaryKey"`
	Name      string    `gorm:"column:name"`
	AppliedAt time.Time `gorm:"column:applied_at"`
}

// TableName 指定表名。
func (schemaMigration) TableName() string { return schemaMigrationsTable }

// Migrate 开发/测试入口：先执行版本化迁移，再用 AutoMigrate 兜底模型漂移。
func Migrate(db *gorm.DB) error { return migrate(db, true) }

// MigrateRelease 生产入口：只执行版本化迁移。
// 已有库首次接入时仍需 AutoMigrate 补齐历史结构（见 migrate 的基线化分支）。
func MigrateRelease(db *gorm.DB) error { return migrate(db, false) }

// AutoMigrate 仅按模型建表/补列（开发、测试与迁移脚本生成器用）。
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(entities()...); err != nil {
		return fmt.Errorf("database: automigrate: %w", err)
	}
	return nil
}

// entities 返回全部 GORM 实体（与迁移脚本的目标结构一致）。
func entities() []any {
	return []any{
		&model.UserInfo{},
		&model.Project{},
		&model.UserProject{},
		&model.Group{},
		&model.Api{},
		&model.ApiReturnSample{},
		&model.Doc{},
		&model.Bug{},
		&model.BugAssign{},
		&model.FieldMapping{},
		&model.VariableEnv{},
		&model.Variable{},
		&model.ShareLink{},
		&model.Property{},
		&model.Message{},
		&model.Notification{},
		&model.FileRecord{},
		&model.OperationLog{},
		// 同步地基（S1/S3/S4/S7）
		&model.ChangeLog{},
		&model.SyncOp{},
		&model.UserToken{},
		// 内容快照（RM-05）
		&model.ContentSnapshot{},
	}
}

// migrate 执行迁移主流程。
func migrate(db *gorm.DB, auto bool) error {
	if err := validateMigrationSets(); err != nil {
		return err
	}
	dialect := db.Dialector.Name()
	if dialect != "sqlite" && dialect != "postgres" {
		return fmt.Errorf("database: 不支持的迁移方言 %q", dialect)
	}
	if err := dropLegacyIndexes(db); err != nil {
		return err
	}
	if err := ensureSchemaMigrations(db); err != nil {
		return err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}
	if len(applied) == 0 && hasLegacySchema(db) {
		if err := baselineLegacy(db, dialect); err != nil {
			return err
		}
		if applied, err = appliedVersions(db); err != nil {
			return err
		}
	}
	if err := applyPending(db, dialect, applied); err != nil {
		return err
	}
	if auto {
		return AutoMigrate(db)
	}
	return nil
}

// baselineLegacy 已有库首次接入：先补模型，再把当前全部脚本记为已应用，
// 避免重跑 0001 的 CREATE TABLE 失败（审核 H-14）。
func baselineLegacy(db *gorm.DB, dialect string) error {
	if err := checkUserUniqueness(db); err != nil {
		return err
	}
	if err := AutoMigrate(db); err != nil {
		return fmt.Errorf("database: baseline: %w", err)
	}
	n, err := baselineAll(db, dialect)
	if err != nil {
		return err
	}
	log.Printf("database: 已有库首次接入迁移执行器，已基线化 %d 个版本（后续版本按序执行）", n)
	return nil
}

// applyPending 校验 H-15 历史数据后按序执行待应用迁移。
func applyPending(db *gorm.DB, dialect string, applied map[int]bool) error {
	pending, err := pendingMigrations(dialect, applied)
	if err != nil {
		return err
	}
	if containsVersion(pending, 19) {
		if err := checkUserUniqueness(db); err != nil {
			return err
		}
	}
	for _, m := range pending {
		if err := applyMigration(db, m); err != nil {
			return err
		}
	}
	return nil
}

// dropLegacyIndexes 删除历史遗留/已改名的索引，由 AutoMigrate 或后续迁移按新名重建。
// 仅 sqlite/postgres 支持 IF EXISTS 与局部索引。
func dropLegacyIndexes(db *gorm.DB) error {
	switch db.Dialector.Name() {
	case "sqlite", "postgres":
		for _, name := range []string{
			legacyProjectTitleIndex,
			legacyUserNameEmailIndex,
			legacyUserNameEmailActive,
			legacySampleProjectIndex,
			legacySampleAPIIndex,
		} {
			if err := db.Exec("DROP INDEX IF EXISTS " + name).Error; err != nil {
				return fmt.Errorf("database: drop legacy index %s: %w", name, err)
			}
		}
	}
	return nil
}

// ensureSchemaMigrations 创建版本记账表。
func ensureSchemaMigrations(db *gorm.DB) error {
	const ddl = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name VARCHAR(200) NOT NULL DEFAULT '',
		applied_at TIMESTAMP NOT NULL
	)`
	if err := db.Exec(ddl).Error; err != nil {
		return fmt.Errorf("database: create schema_migrations: %w", err)
	}
	return nil
}

// appliedVersions 返回已应用版本集合。
func appliedVersions(db *gorm.DB) (map[int]bool, error) {
	var rows []schemaMigration
	if err := db.Table(schemaMigrationsTable).Select("version").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("database: read schema_migrations: %w", err)
	}
	out := make(map[int]bool, len(rows))
	for _, r := range rows {
		out[r.Version] = true
	}
	return out, nil
}

// baselineAll 把当前方言的全部版本记为已应用（老库首次接入用）。
func baselineAll(db *gorm.DB, dialect string) (int, error) {
	all, err := loadMigrations(dialect)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	rows := make([]schemaMigration, 0, len(all))
	for _, v := range sortedVersions(all) {
		rows = append(rows, schemaMigration{Version: v, Name: all[v].Name, AppliedAt: now})
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
		return 0, fmt.Errorf("database: baseline schema_migrations: %w", err)
	}
	return len(rows), nil
}

// pendingMigrations 返回尚未应用的版本（升序）。
func pendingMigrations(dialect string, applied map[int]bool) ([]migration, error) {
	all, err := loadMigrations(dialect)
	if err != nil {
		return nil, err
	}
	out := make([]migration, 0, len(all))
	for _, v := range sortedVersions(all) {
		if !applied[v] {
			out = append(out, all[v])
		}
	}
	return out, nil
}

// applyMigration 在单事务内执行 up 脚本并记录版本。
func applyMigration(db *gorm.DB, m migration) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := execScript(tx, m.Up); err != nil {
			return fmt.Errorf("database: migration %04d_%s up: %w", m.Version, m.Name, err)
		}
		row := schemaMigration{Version: m.Version, Name: m.Name, AppliedAt: time.Now()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return fmt.Errorf("database: record migration %04d: %w", m.Version, err)
		}
		return nil
	})
}

// MigrateDown 回滚最近 steps 个已应用迁移（steps<=0 表示全部回滚）。
// 供测试与运维使用；每个版本在一个事务内执行 down 并删除记账行。
func MigrateDown(db *gorm.DB, steps int) error {
	dialect := db.Dialector.Name()
	all, err := loadMigrations(dialect)
	if err != nil {
		return err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}
	versions := make([]int, 0, len(applied))
	for v := range applied {
		versions = append(versions, v)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(versions)))
	if steps > 0 && len(versions) > steps {
		versions = versions[:steps]
	}
	for _, v := range versions {
		m, ok := all[v]
		if !ok {
			return fmt.Errorf("database: schema_migrations 含未知版本 %d", v)
		}
		if err := rollbackMigration(db, m); err != nil {
			return err
		}
	}
	return nil
}

// rollbackMigration 在单事务内执行 down 并删除记账行。
func rollbackMigration(db *gorm.DB, m migration) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := execScript(tx, m.Down); err != nil {
			return fmt.Errorf("database: migration %04d_%s down: %w", m.Version, m.Name, err)
		}
		if err := tx.Exec("DELETE FROM schema_migrations WHERE version = ?", m.Version).Error; err != nil {
			return fmt.Errorf("database: unrecord migration %04d: %w", m.Version, err)
		}
		return nil
	})
}

// hasLegacySchema 判断库是否已有业务表（用于区分「已有库」与「全新库」）。
func hasLegacySchema(db *gorm.DB) bool {
	return db.Migrator().HasTable("user_info")
}

// containsVersion 判断待应用列表里是否包含某版本。
func containsVersion(pending []migration, version int) bool {
	for _, m := range pending {
		if m.Version == version {
			return true
		}
	}
	return false
}

// checkUserUniqueness 在 H-15 单列唯一索引落地前检查历史脏数据。
// 直接让 CREATE UNIQUE INDEX 失败也可以，但错误信息对运维不友好。
func checkUserUniqueness(db *gorm.DB) error {
	if !db.Migrator().HasTable("user_info") {
		return nil
	}
	var dupName struct {
		Name string
		N    int64
	}
	if err := db.Raw(`SELECT name, COUNT(*) AS n FROM user_info
		WHERE is_deleted = 0 GROUP BY name HAVING COUNT(*) > 1 LIMIT 1`).Scan(&dupName).Error; err != nil {
		return fmt.Errorf("database: 检查同名用户: %w", err)
	}
	if dupName.N > 1 {
		return fmt.Errorf("database: user_info 存在同名活跃用户 %q（%d 条），请先清理重复账号后再启动（审核 H-15）", dupName.Name, dupName.N)
	}
	var dupEmail struct {
		Email string
		N     int64
	}
	if err := db.Raw(`SELECT email, COUNT(*) AS n FROM user_info
		WHERE is_deleted = 0 AND email <> '' GROUP BY email HAVING COUNT(*) > 1 LIMIT 1`).Scan(&dupEmail).Error; err != nil {
		return fmt.Errorf("database: 检查重复邮箱: %w", err)
	}
	if dupEmail.N > 1 {
		return fmt.Errorf("database: user_info 存在重复邮箱 %q（%d 条），请先清理重复账号后再启动（审核 H-15）", dupEmail.Email, dupEmail.N)
	}
	return nil
}

// loadMigrations 读取某方言的全部脚本，并校验 up/down 成对、版本与简述不冲突。
func loadMigrations(dialect string) (map[int]migration, error) {
	entries, err := fs.ReadDir(migrations.FS, dialect)
	if err != nil {
		return nil, fmt.Errorf("database: 读取迁移目录 %s: %w", dialect, err)
	}
	out := make(map[int]migration, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationFilePattern.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("database: 非法迁移文件名 %s/%s（应为 <4位版本号>_<简述>.<up|down>.sql）", dialect, e.Name())
		}
		ver, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("database: 版本号非法 %s: %w", m[1], err)
		}
		raw, err := fs.ReadFile(migrations.FS, path.Join(dialect, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("database: 读取 %s/%s: %w", dialect, e.Name(), err)
		}
		cur := out[ver]
		if cur.Name != "" && cur.Name != m[2] {
			return nil, fmt.Errorf("database: 版本 %d 的简述不一致：%s vs %s", ver, cur.Name, m[2])
		}
		cur.Version, cur.Name = ver, m[2]
		switch m[3] {
		case "up":
			if cur.Up != "" {
				return nil, fmt.Errorf("database: 版本 %d 存在重复的 up 脚本", ver)
			}
			cur.Up = string(raw)
		case "down":
			if cur.Down != "" {
				return nil, fmt.Errorf("database: 版本 %d 存在重复的 down 脚本", ver)
			}
			cur.Down = string(raw)
		}
		out[ver] = cur
	}
	for ver, m := range out {
		if m.Up == "" {
			return nil, fmt.Errorf("database: 版本 %04d_%s 缺少 up 脚本", ver, m.Name)
		}
		if m.Down == "" {
			return nil, fmt.Errorf("database: 版本 %04d_%s 缺少 down 脚本", ver, m.Name)
		}
	}
	return out, nil
}

// validateMigrationSets 两个方言的版本号与简述必须一一对应。
func validateMigrationSets() error {
	sq, err := loadMigrations("sqlite")
	if err != nil {
		return err
	}
	pg, err := loadMigrations("postgres")
	if err != nil {
		return err
	}
	if len(sq) != len(pg) {
		return fmt.Errorf("database: 两方言迁移数量不一致：sqlite=%d postgres=%d", len(sq), len(pg))
	}
	for v, m := range sq {
		pm, ok := pg[v]
		if !ok {
			return fmt.Errorf("database: 版本 %04d_%s 只在 sqlite 存在", v, m.Name)
		}
		if pm.Name != m.Name {
			return fmt.Errorf("database: 版本 %d 两方言简述不一致：sqlite=%s postgres=%s", v, m.Name, pm.Name)
		}
	}
	return nil
}

// sortedVersions 返回升序版本号。
func sortedVersions(all map[int]migration) []int {
	versions := make([]int, 0, len(all))
	for v := range all {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	return versions
}

// execScript 按语句拆分并逐条执行。
// 不能把整份脚本一次性交给驱动：PostgreSQL 的扩展协议不接受多语句，
// 而 SQLite 的出错位置也更难定位。
func execScript(tx *gorm.DB, script string) error {
	for _, stmt := range splitSQL(script) {
		if err := tx.Exec(stmt).Error; err != nil {
			return fmt.Errorf("%w\nSQL: %s", err, truncateSQL(stmt))
		}
	}
	return nil
}

// splitSQL 按分号拆分 SQL，跳过字符串、标识符、dollar-quoted 块与注释中的分号。
func splitSQL(script string) []string {
	var stmts []string
	var b strings.Builder
	var quote byte
	var dollarTag string
	inLine, inBlock := false, false
	flush := func() {
		stmt := strings.TrimSpace(b.String())
		b.Reset()
		if stmt != "" {
			stmts = append(stmts, stmt)
		}
	}
	for i := 0; i < len(script); i++ {
		c := script[i]
		switch {
		case inLine:
			if c == '\n' {
				inLine = false
				b.WriteByte(c)
			}
		case inBlock:
			if c == '*' && i+1 < len(script) && script[i+1] == '/' {
				inBlock = false
				i++
			}
		case dollarTag != "":
			if strings.HasPrefix(script[i:], dollarTag) {
				b.WriteString(dollarTag)
				i += len(dollarTag) - 1
				dollarTag = ""
				continue
			}
			b.WriteByte(c)
		case quote != 0:
			b.WriteByte(c)
			if c == quote {
				if i+1 < len(script) && script[i+1] == quote {
					b.WriteByte(script[i+1])
					i++
					continue
				}
				quote = 0
			}
		case c == '-' && i+1 < len(script) && script[i+1] == '-':
			inLine = true
			i++
		case c == '/' && i+1 < len(script) && script[i+1] == '*':
			inBlock = true
			i++
		case c == '\'' || c == '"' || c == '`':
			quote = c
			b.WriteByte(c)
		case c == '$':
			if tag, n := dollarTagAt(script[i:]); tag != "" {
				dollarTag = tag
				b.WriteString(tag)
				i += n - 1
			} else {
				b.WriteByte(c)
			}
		case c == ';':
			flush()
		default:
			b.WriteByte(c)
		}
	}
	flush()
	return stmts
}

// dollarTagAt 识别 PostgreSQL 的 $tag$ 定界符；不是合法标签时返回空。
func dollarTagAt(s string) (string, int) {
	if len(s) < 2 || s[0] != '$' {
		return "", 0
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c == '$' {
			return s[:i+1], i + 1
		}
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return "", 0
		}
	}
	return "", 0
}

// truncateSQL 把语句压成单行并截断，便于错误日志定位。
func truncateSQL(stmt string) string {
	s := strings.Join(strings.Fields(stmt), " ")
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}
