
-- 0008_share_and_doc_icon：① 文档支持图标（上传图片 URL 或 emoji）② 新增免登录只读分享链接表。
-- 与 PostgreSQL 版 migrations/postgres/0008_share_and_doc_icon.up.sql 等价。
ALTER TABLE `doc` ADD COLUMN `icon` text;
CREATE TABLE `share_link` (`id` integer PRIMARY KEY AUTOINCREMENT,`is_deleted` integer NOT NULL DEFAULT 0,`create_time` datetime,`update_time` datetime,`project_id` integer NOT NULL,`user_id` integer NOT NULL DEFAULT 0,`token` text NOT NULL,`remark` text,`expire_time` datetime);
CREATE INDEX `idx_share_link_project_id` ON `share_link`(`project_id`);
CREATE UNIQUE INDEX `uk_share_token` ON `share_link`(`token`) WHERE `is_deleted` = 0;
