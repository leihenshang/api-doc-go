-- 0015 回滚：恢复不区分软删除的全局唯一索引。
-- 注意：若库里存在多条 is_deleted = 1 且 name/email 相同的记录，回滚会失败，需先物理清理。
DROP INDEX IF EXISTS "uk_name_email_active";
CREATE UNIQUE INDEX IF NOT EXISTS "uk_name_email" ON "user_info"("name", "email");
