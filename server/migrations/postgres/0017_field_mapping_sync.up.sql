-- 0017_field_mapping_sync：field_mapping 加同步身份列（ext_uid / last_change_id），
-- 用于项目字段字典双向同步（客户端集合内 field_mappings.yml ↔ 服务端 field_mapping 表）。
-- 等价 SQLite 版 migrations/sqlite/0017_field_mapping_sync.up.sql。
ALTER TABLE field_mapping ADD COLUMN "ext_uid" VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE field_mapping ADD COLUMN "last_change_id" BIGINT NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS uk_field_mapping_ext_uid ON field_mapping("ext_uid") WHERE is_deleted = 0 AND ext_uid <> '';
