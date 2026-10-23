-- 0019_user_name_email_unique：name / email 拆成两个独立的局部唯一索引（审核 H-15）。
-- 等价 SQLite 版 migrations/sqlite/0019_user_name_email_unique.up.sql。
-- 注意：若库中已存在同名或同邮箱的活跃用户，本迁移会因唯一约束失败，需先人工清理重复账号。
DROP INDEX IF EXISTS "uk_name_email";
DROP INDEX IF EXISTS "uk_name_email_active";

CREATE UNIQUE INDEX IF NOT EXISTS "uk_user_name_active"
    ON "user_info"("name") WHERE is_deleted = 0;
CREATE UNIQUE INDEX IF NOT EXISTS "uk_user_email_active"
    ON "user_info"("email") WHERE is_deleted = 0 AND email <> '';
