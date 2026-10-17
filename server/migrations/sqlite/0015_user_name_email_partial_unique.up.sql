-- 0015_user_name_email_partial_unique：user_info 的 name/email 唯一索引改为局部唯一
-- 旧索引 uk_name_email(name, email) 不区分软删除，软删用户会永久占用 name/email：
-- 应用层 UserTaken 走 active()（排除 is_deleted=1）判定「可用」，插入却撞 DB 约束。
-- 新索引加 WHERE is_deleted = 0，与 model 的 `where:` tag 完全一致。

DROP INDEX IF EXISTS uk_name_email;
CREATE UNIQUE INDEX IF NOT EXISTS uk_name_email_active
    ON user_info(name, email) WHERE is_deleted = 0;
