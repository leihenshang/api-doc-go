-- 0012_api_return_sample：接口多响应示例表，承载客户端保存的命名响应示例，支持同步。
CREATE TABLE api_return_sample (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL DEFAULT 0,
    api_id BIGINT NOT NULL DEFAULT 0,
    ext_uid VARCHAR(64) NOT NULL DEFAULT '',
    last_change_id BIGINT NOT NULL DEFAULT 0,
    name VARCHAR(500) NOT NULL DEFAULT '',
    sample_text TEXT,
    status INTEGER NOT NULL DEFAULT 200,
    content_type VARCHAR(200) NOT NULL DEFAULT '',
    seq INTEGER NOT NULL DEFAULT 0,
    is_deleted SMALLINT NOT NULL DEFAULT 0,
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_api_return_sample_project ON api_return_sample(project_id);
CREATE INDEX idx_api_return_sample_api ON api_return_sample(api_id);
CREATE UNIQUE INDEX uk_api_sample_ext_uid ON api_return_sample(ext_uid) WHERE is_deleted = 0 AND ext_uid <> '';
