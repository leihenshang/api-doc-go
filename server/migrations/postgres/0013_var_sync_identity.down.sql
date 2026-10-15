-- 0013 回滚：移除 variable / variable_env 的同步身份列。
DROP INDEX IF EXISTS uk_variable_ext_uid;
DROP INDEX IF EXISTS uk_var_env_ext_uid;
ALTER TABLE "variable" DROP COLUMN IF EXISTS "ext_uid";
ALTER TABLE "variable" DROP COLUMN IF EXISTS "last_change_id";
ALTER TABLE "variable_env" DROP COLUMN IF EXISTS "ext_uid";
ALTER TABLE "variable_env" DROP COLUMN IF EXISTS "last_change_id";
