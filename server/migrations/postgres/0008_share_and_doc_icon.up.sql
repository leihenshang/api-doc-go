-- 0008_share_and_doc_icon：① 文档支持图标（上传图片 URL 或 emoji）② 新增免登录只读分享链接表。
-- 与 SQLite 版 migrations/sqlite/0008_share_and_doc_icon.up.sql 等价。
ALTER TABLE "doc" ADD COLUMN "icon" text;

CREATE TABLE "share_link" (
  "id" bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  "is_deleted" smallint NOT NULL DEFAULT 0,
  "create_time" timestamptz, "update_time" timestamptz,
  "project_id" bigint NOT NULL,
  "user_id" bigint NOT NULL DEFAULT 0,
  "token" text NOT NULL, "remark" text, "expire_time" timestamptz
);
CREATE INDEX "idx_share_link_project_id" ON "share_link"("project_id");
CREATE UNIQUE INDEX "uk_share_token" ON "share_link"("token") WHERE "is_deleted" = 0;
