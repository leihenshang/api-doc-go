-- 0014 回滚：移除 variable / variable_env 的同步身份列。
DROP INDEX IF EXISTS uk_variable_ext_uid;
DROP INDEX IF EXISTS uk_var_env_ext_uid;
ALTER TABLE variable DROP COLUMN ext_uid;
ALTER TABLE variable DROP COLUMN last_change_id;
ALTER TABLE variable_env DROP COLUMN ext_uid;
ALTER TABLE variable_env DROP COLUMN last_change_id;
