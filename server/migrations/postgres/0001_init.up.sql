-- 0001_init：api-doc 初始结构（PostgreSQL）。
-- 与 SQLite 版 migrations/sqlite/0001_init.up.sql 等价；列/索引命名与 model GORM tag 保持一致。
-- 类型映射：AUTOINCREMENT→IDENTITY，datetime→timestamptz，Go int8→smallint，Go uint64→bigint。

CREATE TABLE "api" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz,
  "update_time" timestamptz,
  "project_id" bigint NOT NULL,
  "group_id" bigint NOT NULL DEFAULT 0,
  "group_id_second" bigint NOT NULL DEFAULT 0,
  "description" text, "protocol_type" text, "http_method_type" text, "url" text,
  "api_name" text, "object_name" text, "function_name" text, "develop_language" text,
  "http_request_header" text, "http_request_params" text, "http_return_type" text,
  "http_return_params" text, "http_return_sample" text
);
CREATE INDEX "idx_api_project_id" ON "api"("project_id");
CREATE INDEX "idx_api_group_id" ON "api"("group_id");

CREATE TABLE "bug" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "project_id" bigint NOT NULL, "to_user_id" bigint NOT NULL DEFAULT 0,
  "title" text NOT NULL, "content" text NOT NULL, "comment" text,
  "status" smallint NOT NULL DEFAULT 1, "level" smallint NOT NULL DEFAULT 1
);
CREATE INDEX "idx_bug_project_id" ON "bug"("project_id");

CREATE TABLE "bug_assign" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "bug_id" bigint NOT NULL, "from_user_id" bigint NOT NULL DEFAULT 0,
  "to_user_id" bigint NOT NULL DEFAULT 0, "comment" text,
  "status" smallint NOT NULL DEFAULT 1
);
CREATE INDEX "idx_bug_assign_bug_id" ON "bug_assign"("bug_id");

CREATE TABLE "doc" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "user_id" bigint NOT NULL, "project_id" bigint NOT NULL, "group_id" bigint NOT NULL DEFAULT 0,
  "title" text NOT NULL, "content" text NOT NULL,
  "state" smallint NOT NULL DEFAULT 0,
  "view_count" integer NOT NULL DEFAULT 0, "like_count" integer NOT NULL DEFAULT 0,
  "is_top" smallint NOT NULL DEFAULT 0, "priority" integer NOT NULL DEFAULT 0
);
CREATE INDEX "idx_doc_user_id" ON "doc"("user_id");
CREATE INDEX "idx_doc_project_id" ON "doc"("project_id");
CREATE INDEX "idx_doc_group_id" ON "doc"("group_id");

CREATE TABLE "field_mapping" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "project_id" bigint NOT NULL, "field" text NOT NULL, "type" text NOT NULL, "description" text
);
CREATE INDEX "idx_field_mapping_project_id" ON "field_mapping"("project_id");

CREATE TABLE "property" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "tag" text NOT NULL, "tag_name" text NOT NULL, "description" text
);

CREATE TABLE "message" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "content" text, "send_type" smallint NOT NULL DEFAULT 0,
  "receive_source" text NOT NULL, "title" text,
  "expire_time" timestamptz NOT NULL, "used_time" timestamptz,
  "is_used" smallint NOT NULL DEFAULT 0, "code" text
);

CREATE TABLE "notification" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "title" text, "content" text, "send_type" smallint NOT NULL DEFAULT 0,
  "receiver" bigint NOT NULL, "is_read" smallint NOT NULL DEFAULT 0
);
CREATE INDEX "idx_notification_receiver" ON "notification"("receiver");

CREATE TABLE "operation_log" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "user_id" bigint NOT NULL, "action" text NOT NULL, "content" text,
  "object_id" bigint NOT NULL DEFAULT 0, "type" smallint NOT NULL DEFAULT 0,
  "project_id" bigint NOT NULL
);
CREATE INDEX "idx_operation_log_user_id" ON "operation_log"("user_id");
CREATE INDEX "idx_operation_log_object_id" ON "operation_log"("object_id");
CREATE INDEX "idx_operation_log_project_id" ON "operation_log"("project_id");

CREATE TABLE "project" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "title" text NOT NULL, "description" text, "version" text, "type" text
);
CREATE UNIQUE INDEX "idx_project_title" ON "project"("title");

CREATE TABLE "user_info" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "name" text NOT NULL, "pwd" text NOT NULL, "email" text,
  "type" smallint NOT NULL DEFAULT 1, "state" smallint NOT NULL DEFAULT 1,
  "mobile_number" text, "nick_name" text, "last_login_ip" text,
  "last_login_time" timestamptz, "user_face" text
);
CREATE UNIQUE INDEX "uk_name_email" ON "user_info"("name", "email");

CREATE TABLE "user_project" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "user_id" bigint NOT NULL, "project_id" bigint NOT NULL,
  "is_leader" smallint NOT NULL DEFAULT 0, "permission" smallint NOT NULL DEFAULT 1
);
CREATE INDEX "idx_user_project" ON "user_project"("user_id", "project_id");

CREATE TABLE "file" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "title" text NOT NULL, "path" text
);

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

CREATE TABLE "group" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "p_id" bigint NOT NULL DEFAULT 0, "project_id" bigint NOT NULL DEFAULT 0,
  "title" text NOT NULL, "priority" integer NOT NULL DEFAULT 0,
  "type" smallint NOT NULL DEFAULT 0
);
CREATE INDEX "idx_group_p_id" ON "group"("p_id");
CREATE INDEX "idx_group_project_id" ON "group"("project_id");
