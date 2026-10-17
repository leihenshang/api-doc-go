-- 0010_sync_foundation：同步地基（ext_uid / last_change_id / change_log / sync_op / user_token / client_write_enabled）。
-- 与 SQLite 版 migrations/sqlite/0010_sync_foundation.up.sql 等价；列名与 model GORM tag 保持一致。
-- 类型映射：AUTOINCREMENT→BIGSERIAL，datetime→timestamptz，Go int8→smallint，Go bool→boolean，Go uint64→bigint。
-- 索引名一律取 GORM 默认命名（idx_<表>_<列>），避免 AutoMigrate 再建出同名以外的重复索引。
-- 局部唯一索引用 is_deleted = 0 AND ext_uid <> ''，与 model 的 `where:` tag 完全一致：
-- 历史行的 ext_uid 为 ''，若只按 is_deleted = 0 过滤，多行 '' 会互撞导致建索引失败。

-- 一、同步实体加列（api / group / doc）
ALTER TABLE "api" ADD COLUMN IF NOT EXISTS "ext_uid" varchar(64) NOT NULL DEFAULT '';
ALTER TABLE "api" ADD COLUMN IF NOT EXISTS "last_change_id" bigint NOT NULL DEFAULT 0;
ALTER TABLE "group" ADD COLUMN IF NOT EXISTS "ext_uid" varchar(64) NOT NULL DEFAULT '';
ALTER TABLE "group" ADD COLUMN IF NOT EXISTS "last_change_id" bigint NOT NULL DEFAULT 0;
ALTER TABLE "doc" ADD COLUMN IF NOT EXISTS "ext_uid" varchar(64) NOT NULL DEFAULT '';
ALTER TABLE "doc" ADD COLUMN IF NOT EXISTS "last_change_id" bigint NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS "uk_api_ext_uid" ON "api"("ext_uid") WHERE is_deleted = 0 AND ext_uid <> '';
CREATE UNIQUE INDEX IF NOT EXISTS "uk_group_ext_uid" ON "group"("ext_uid") WHERE is_deleted = 0 AND ext_uid <> '';
CREATE UNIQUE INDEX IF NOT EXISTS "uk_doc_ext_uid" ON "doc"("ext_uid") WHERE is_deleted = 0 AND ext_uid <> '';

-- 二、项目写开关（S14）：关闭后 /api/sync/push 一律 rejected.readonly
ALTER TABLE "project" ADD COLUMN IF NOT EXISTS "client_write_enabled" boolean NOT NULL DEFAULT true;

-- 三、变更流水（S3）：项目级游标即 change_log.id
CREATE TABLE IF NOT EXISTS "change_log" (
  "id" bigserial PRIMARY KEY,
  "project_id" bigint NOT NULL,
  "entity_type" varchar(20) NOT NULL,
  "ext_uid" varchar(64) NOT NULL,
  "op" varchar(10) NOT NULL,
  "actor" bigint NOT NULL DEFAULT 0,
  "device" varchar(100) NOT NULL DEFAULT '',
  "create_time" timestamptz
);
CREATE INDEX IF NOT EXISTS "idx_change_log_project_id" ON "change_log"("project_id");
CREATE INDEX IF NOT EXISTS "idx_change_log_ext_uid" ON "change_log"("ext_uid");

-- 四、push 幂等键（S4/S5）：同一 op_id 只处理一次
CREATE TABLE IF NOT EXISTS "sync_op" (
  "op_id" varchar(64) PRIMARY KEY,
  "project_id" bigint NOT NULL,
  "status" varchar(20) NOT NULL,
  "item_rev" bigint NOT NULL DEFAULT 0,
  "reason" varchar(200) NOT NULL DEFAULT '',
  "create_time" timestamptz
);
CREATE INDEX IF NOT EXISTS "idx_sync_op_project_id" ON "sync_op"("project_id");

-- 五、个人访问令牌（S7）；token_cipher 由后续 0011_token_cipher 补列
CREATE TABLE IF NOT EXISTS "user_token" (
  "id" bigserial PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz,
  "update_time" timestamptz,
  "user_id" bigint NOT NULL,
  "name" varchar(100) NOT NULL,
  "token_hash" varchar(128) NOT NULL,
  "scopes" varchar(200) NOT NULL DEFAULT '',
  "expire_at" timestamptz,
  "last_used" timestamptz,
  "revoked" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "uk_token_hash" ON "user_token"("token_hash");
CREATE INDEX IF NOT EXISTS "idx_user_token_user_id" ON "user_token"("user_id");
