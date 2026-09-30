-- 0010_sync_foundation: 同步地基（ext_uid / last_change_id / change_log / sync_op / user_token / client_write_enabled）

-- 同步实体加列
ALTER TABLE api ADD COLUMN ext_uid VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE api ADD COLUMN last_change_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE "group" ADD COLUMN ext_uid VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE "group" ADD COLUMN last_change_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE doc ADD COLUMN ext_uid VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE doc ADD COLUMN last_change_id BIGINT NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS uk_api_ext_uid ON api(ext_uid) WHERE is_deleted = 0;
CREATE UNIQUE INDEX IF NOT EXISTS uk_group_ext_uid ON "group"(ext_uid) WHERE is_deleted = 0;
CREATE UNIQUE INDEX IF NOT EXISTS uk_doc_ext_uid ON doc(ext_uid) WHERE is_deleted = 0;

-- 项目写开关（S14）
ALTER TABLE project ADD COLUMN client_write_enabled SMALLINT NOT NULL DEFAULT 1;

-- 变更流水（S3）
CREATE TABLE IF NOT EXISTS change_log (
    id BIGINT PRIMARY KEY AUTOINCREMENT,
    project_id BIGINT NOT NULL,
    entity_type VARCHAR(20) NOT NULL,
    ext_uid VARCHAR(64) NOT NULL,
    op VARCHAR(10) NOT NULL,
    actor BIGINT NOT NULL DEFAULT 0,
    device VARCHAR(100) NOT NULL DEFAULT '',
    create_time DATETIME
);
CREATE INDEX IF NOT EXISTS idx_change_log_project ON change_log(project_id);
CREATE INDEX IF NOT EXISTS idx_change_log_ext_uid ON change_log(ext_uid);

-- push 幂等（S4/S5）
CREATE TABLE IF NOT EXISTS sync_op (
    op_id VARCHAR(64) PRIMARY KEY,
    project_id BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL,
    item_rev BIGINT NOT NULL DEFAULT 0,
    reason VARCHAR(200) NOT NULL DEFAULT '',
    create_time DATETIME
);
CREATE INDEX IF NOT EXISTS idx_sync_op_project ON sync_op(project_id);

-- PAT（S7）
CREATE TABLE IF NOT EXISTS user_token (
    id BIGINT PRIMARY KEY AUTOINCREMENT,
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
CREATE INDEX IF NOT EXISTS idx_user_token_user ON user_token(user_id);
