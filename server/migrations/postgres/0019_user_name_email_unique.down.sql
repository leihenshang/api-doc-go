-- 0019 回滚：恢复 0015 的复合唯一索引 (name, email)。
-- 注意：若存在同名或同邮箱的活跃用户，回滚会失败，需先人工清理。
DROP INDEX IF EXISTS "uk_user_name_active";
DROP INDEX IF EXISTS "uk_user_email_active";

CREATE UNIQUE INDEX IF NOT EXISTS "uk_name_email_active"
    ON "user_info"("name", "email") WHERE is_deleted = 0;
