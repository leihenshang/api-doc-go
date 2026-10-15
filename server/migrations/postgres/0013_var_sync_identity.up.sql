-- 0013_var_sync_identity：variable / variable_env 加同步身份列（ext_uid / last_change_id），
-- 用于变量类同步（S16）；secret 变量值只同步客户端密文（enc:v1: 前缀），服务端只存不解析。
-- 等价 SQLite 版 migrations/sqlite/0014_var_sync_identity.up.sql。
ALTER TABLE "variable" ADD COLUMN "ext_uid" VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE "variable" ADD COLUMN "last_change_id" BIGINT NOT NULL DEFAULT 0;
ALTER TABLE "variable_env" ADD COLUMN "ext_uid" VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE "variable_env" ADD COLUMN "last_change_id" BIGINT NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS uk_variable_ext_uid ON "variable"("ext_uid") WHERE is_deleted = 0 AND ext_uid <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uk_var_env_ext_uid ON "variable_env"("ext_uid") WHERE is_deleted = 0 AND ext_uid <> '';
