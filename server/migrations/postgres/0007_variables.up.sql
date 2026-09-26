-- 0007_variables：新增项目级环境表 variable_env 与三级变量表 variable（请求级/项目级/全局）。
-- 变量作用域 scope：1 请求级（绑定 api_id）/ 2 项目级（env_id=0 通用、>0 绑定环境）/ 3 全局（应用级，跨项目共享）。
-- 同层同环境不允许重名：uk_variable_key 为排除软删除行的局部唯一索引。
-- 与 SQLite 版 migrations/sqlite/0007_variables.up.sql 等价。
CREATE TABLE "variable_env" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "project_id" bigint NOT NULL, "name" text NOT NULL, "description" text, "color" text,
  "is_default" smallint NOT NULL DEFAULT 0, "priority" integer NOT NULL DEFAULT 0
);
CREATE INDEX "idx_variable_env_project_id" ON "variable_env"("project_id");
CREATE UNIQUE INDEX "uk_var_env_name" ON "variable_env"("project_id", "name") WHERE "is_deleted" = 0;

CREATE TABLE "variable" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "scope" smallint NOT NULL DEFAULT 1,
  "project_id" bigint NOT NULL DEFAULT 0,
  "api_id" bigint NOT NULL DEFAULT 0,
  "env_id" bigint NOT NULL DEFAULT 0,
  "name" text NOT NULL, "value" text, "description" text,
  "is_secret" smallint NOT NULL DEFAULT 0, "enabled" smallint NOT NULL DEFAULT 1,
  "priority" integer NOT NULL DEFAULT 0
);
CREATE INDEX "idx_variable_scope" ON "variable"("project_id", "api_id");
CREATE INDEX "idx_variable_name" ON "variable"("name");
CREATE UNIQUE INDEX "uk_variable_key" ON "variable"("scope", "project_id", "api_id", "env_id", "name") WHERE "is_deleted" = 0;
