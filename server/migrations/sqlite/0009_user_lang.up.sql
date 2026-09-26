-- 0009_user_lang：用户界面语言偏好（user_info.lang，zh-CN / en-US，空串=未选择）。
-- 与 PostgreSQL 版 migrations/postgres/0009_user_lang.up.sql 等价。
ALTER TABLE `user_info` ADD COLUMN `lang` text NOT NULL DEFAULT '';
