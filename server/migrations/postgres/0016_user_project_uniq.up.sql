-- 0016_user_project_uniq：成员关系加 (project_id, user_id) 部分唯一索引（见 sqlite 版说明）。
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_project
    ON user_project(user_id, project_id) WHERE is_deleted = 0;
