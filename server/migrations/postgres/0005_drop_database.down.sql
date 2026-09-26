-- 0005 回滚：重建连接表（结构同 0001_init）。
CREATE TABLE "database" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "address" text NOT NULL, "port" integer NOT NULL DEFAULT 0,
  "username" text NOT NULL, "password" text NOT NULL, "database_name" text NOT NULL,
  "project_id" bigint NOT NULL, "user_id" bigint NOT NULL DEFAULT 0,
  "type" smallint NOT NULL DEFAULT 1, "description" text
);
CREATE INDEX "idx_database_project_id" ON "database"("project_id");
