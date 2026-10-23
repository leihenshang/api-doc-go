# 数据库迁移

结构变更以带版本号的 SQL 文件维护，SQLite 与 PostgreSQL 各一套（见开发规范 §6.5）。

## 约定

- 文件名：`<版本号>_<简述>.<up|down>.sql`，如 `0001_init.up.sql` / `0001_init.down.sql`
- 目录：`sqlite/` 与 `postgres/` 分别存放对应方言脚本
- **每个版本必须 up/down 成对**；两个方言的版本号与简述必须一致（启动时校验）
- **禁止只改代码不补迁移**：`TestMigrationsMatchModels` 会对比「迁移建出的结构」与
  「AutoMigrate 建出的结构」，缺失列/索引会直接失败
- 一个版本的 up/down 各自在**单事务**内执行，禁止在脚本里写 `BEGIN` / `COMMIT`
- 语句按分号拆分执行；字符串、标识符、`$tag$` 块与注释中的分号不会被误拆

## 执行方式

- 脚本由 `migrations/embed.go` 以 `embed.FS` 打包进二进制。
- 启动时 `internal/database` 的迁移执行器按版本号升序执行，并写入 `schema_migrations`。
- **生产**（`run_mode=release`）走 `database.MigrateRelease`：只执行版本化迁移。
- **开发/测试**走 `database.Migrate`：迁移后再 `AutoMigrate` 兜底模型漂移。
- **已有库首次接入**（已有业务表、尚无 `schema_migrations`）：先 `AutoMigrate` 补齐结构，
  再把当前全部版本记为已应用（基线化），不会重跑 `0001_init` 的 `CREATE TABLE`。
- 回滚：`database.MigrateDown(db, steps)`（`steps<=0` 表示全部回滚），供测试与运维使用。

## PostgreSQL CI

`.github/workflows/ci.yml` 的 `migrate-postgres` job 使用 `postgres:16` service，
设置 `TEST_POSTGRES_DSN` 后运行 `go test ./internal/database -run TestPostgresMigrationsApply`，
在真实 PG 上验证全部 up 与 down。本地未设置该环境变量时测试自动跳过。

> 不做旧项目历史数据迁移，新库通过此目录全新建表 + 必要种子数据（如超级管理员）。
