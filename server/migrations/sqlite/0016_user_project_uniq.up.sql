-- 0016_user_project_uniq：成员关系加 (project_id, user_id) 部分唯一索引。
-- 此前只有普通联合索引 idx_user_project，两个并发请求同时添加同一成员会各插一行，
-- 之后按 user_id 单查命中哪条不确定，权限判定结果不可预测。
-- 用 WHERE is_deleted = 0 与 model tag 一致：被软删的旧成员行不应阻止重新添加。

CREATE UNIQUE INDEX IF NOT EXISTS uk_user_project
    ON user_project(user_id, project_id) WHERE is_deleted = 0;
