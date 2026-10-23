-- 0018 回滚：移除补齐列/索引，并恢复 api_return_sample 的历史索引名。
DROP INDEX IF EXISTS idx_file_hash;
DROP INDEX IF EXISTS idx_api_return_sample_project_id;
DROP INDEX IF EXISTS idx_api_return_sample_api_id;
CREATE INDEX IF NOT EXISTS idx_api_return_sample_project ON api_return_sample(project_id);
CREATE INDEX IF NOT EXISTS idx_api_return_sample_api ON api_return_sample(api_id);

ALTER TABLE file DROP COLUMN IF EXISTS mime;
ALTER TABLE file DROP COLUMN IF EXISTS size;
ALTER TABLE file DROP COLUMN IF EXISTS hash;
