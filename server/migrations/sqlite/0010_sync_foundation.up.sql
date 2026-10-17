-- 0010_sync_foundation：同步地基（ext_uid / last_change_id / change_log / sync_op / user_token / client_write_enabled）。
-- 索引名一律取 GORM 默认命名（idx_<表>_<列>），与 AutoMigrate 生成的名字一致，避免同一列上出现两个重复索引。
-- 局部唯一索引用 is_deleted = 0 AND ext_uid <> ''，与 model 的 `where:` tag 完全一致：
-- 历史行的 ext_uid 为 ''，若只按 is_deleted = 0 过滤，多行 '' 会互撞导致建索引失败。
-- 注意：SQLite 要求自增主键必须是 `INTEGER PRIMARY KEY AUTOINCREMENT`，写成 BIGINT 会直接报
-- "AUTOINCREMENT is only allowed on an INTEGER PRIMARY KEY"。

-- 一、同步实体加列（api / group / doc）
ALTER TABLE api ADD COLUMN ext_uid VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE api ADD COLUMN last_change_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE "group" ADD COLUMN ext_uid VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE "group" ADD COLUMN last_change_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE doc ADD COLUMN ext_uid VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE doc ADD COLUMN last_change_id BIGINT NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS uk_api_ext_uid ON api(ext_uid) WHERE is_deleted = 0 AND ext_uid <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uk_group_ext_uid ON "group"(ext_uid) WHERE is_deleted = 0 AND ext_uid <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uk_doc_ext_uid ON doc(ext_uid) WHERE is_deleted = 0 AND ext_uid <> '';

-- 二、项目写开关（S14）：关闭后 /api/sync/push 一律 rejected.readonly
ALTER TABLE project ADD COLUMN client_write_enabled SMALLINT NOT NULL DEFAULT 1;

-- 三、变更流水（S3）：项目级游标即 change_log.id
CREATE TABLE IF NOT EXISTS change_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id BIGINT NOT NULL,
    entity_type VARCHAR(20) NOT NULL,
    ext_uid VARCHAR(64) NOT NULL,
    op VARCHAR(10) NOT NULL,
    actor BIGINT NOT NULL DEFAULT 0,
    device VARCHAR(100) NOT NULL DEFAULT '',
    create_time DATETIME
);
CREATE INDEX IF NOT EXISTS idx_change_log_project_id ON change_log(project_id);
CREATE INDEX IF NOT EXISTS idx_change_log_ext_uid ON change_log(ext_uid);

-- 四、push 幂等键（S4/S5）：同一 op_id 只处理一次
CREATE TABLE IF NOT EXISTS sync_op (
    op_id VARCHAR(64) PRIMARY KEY,
    project_id BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL,
    item_rev BIGINT NOT NULL DEFAULT 0,
    reason VARCHAR(200) NOT NULL DEFAULT '',
    create_time DATETIME
);
CREATE INDEX IF NOT EXISTS idx_sync_op_project_id ON sync_op(project_id);

-- 五、个人访问令牌（S7）；token_cipher 由后续 0011_token_cipher 补列
CREATE TABLE IF NOT EXISTS user_token (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    is_deleted SMALLINT NOT NULL DEFAULT 0,
    create_time DATETIME,
    update_time DATETIME,
    user_id BIGINT NOT NULL,
    name VARCHAR(100) NOT NULL,
    token_hash VARCHAR(128) NOT NULL,
    scopes VARCHAR(200) NOT NULL DEFAULT '',
    expire_at DATETIME,
    last_used DATETIME,
    revoked SMALLINT NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_token_hash ON user_token(token_hash);
CREATE INDEX IF NOT EXISTS idx_user_token_user_id ON user_token(user_id);
