-- 0017 回滚：移除 field_mapping 的同步身份列。
DROP INDEX IF EXISTS uk_field_mapping_ext_uid;
ALTER TABLE field_mapping DROP COLUMN ext_uid;
ALTER TABLE field_mapping DROP COLUMN last_change_id;
