-- 0003_project_title_partial_unique：项目名称唯一改为「排除软删除行」的局部唯一索引。
-- 背景：原 `idx_project_title` 不区分软删除，删除项目后同名项目无法再创建（列表里看不到却报重名）。
-- 与 SQLite 版 migrations/sqlite/0003_project_title_partial_unique.up.sql 等价。
DROP INDEX IF EXISTS "idx_project_title";
CREATE UNIQUE INDEX "uk_project_title" ON "project"("title") WHERE "is_deleted" = 0;
