-- 0003 回滚：恢复不区分软删除的全局唯一索引（回滚前需清理重名项目，否则创建索引会失败）。
DROP INDEX IF EXISTS `uk_project_title`;
CREATE UNIQUE INDEX `idx_project_title` ON `project`(`title`);
