-- 0018_schema_parity：补齐"模型已有、迁移缺失"的结构，并统一索引命名（审核 H-14）。
-- 背景：file.hash/size/mime 由模型新增后一直靠 AutoMigrate 补列，迁移脚本里没有；
-- api_return_sample 的索引名也与 GORM 默认命名（idx_<表>_<列>）不一致。
ALTER TABLE file ADD COLUMN hash VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE file ADD COLUMN size BIGINT NOT NULL DEFAULT 0;
ALTER TABLE file ADD COLUMN mime VARCHAR(100) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_file_hash ON file(hash);

DROP INDEX IF EXISTS idx_api_return_sample_project;
DROP INDEX IF EXISTS idx_api_return_sample_api;
CREATE INDEX IF NOT EXISTS idx_api_return_sample_project_id ON api_return_sample(project_id);
CREATE INDEX IF NOT EXISTS idx_api_return_sample_api_id ON api_return_sample(api_id);
