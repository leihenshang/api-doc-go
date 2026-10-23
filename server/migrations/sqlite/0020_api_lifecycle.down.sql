-- 0020 回滚：移除接口生命周期字段。
DROP INDEX IF EXISTS idx_api_status;
ALTER TABLE api DROP COLUMN deprecated_at;
ALTER TABLE api DROP COLUMN version;
ALTER TABLE api DROP COLUMN status;
