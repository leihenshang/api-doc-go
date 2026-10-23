-- 0010 回滚：移除同步地基（索引随表/列一并删除）。
-- 注意：SQLite 的 DROP COLUMN 要求列上不再有索引，故先删索引再删列。
DROP INDEX IF EXISTS uk_api_ext_uid;
DROP INDEX IF EXISTS uk_group_ext_uid;
DROP INDEX IF EXISTS uk_doc_ext_uid;

DROP TABLE IF EXISTS change_log;
DROP TABLE IF EXISTS sync_op;
DROP TABLE IF EXISTS user_token;

ALTER TABLE api DROP COLUMN last_change_id;
ALTER TABLE api DROP COLUMN ext_uid;
ALTER TABLE "group" DROP COLUMN last_change_id;
ALTER TABLE "group" DROP COLUMN ext_uid;
ALTER TABLE doc DROP COLUMN last_change_id;
ALTER TABLE doc DROP COLUMN ext_uid;

ALTER TABLE project DROP COLUMN client_write_enabled;
