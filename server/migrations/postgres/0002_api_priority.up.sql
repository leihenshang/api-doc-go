-- 0002_api_priority：接口新增排序字段（拖拽排序用）。
-- 与 SQLite 版 migrations/sqlite/0002_api_priority.up.sql 等价。
ALTER TABLE "api" ADD COLUMN "priority" integer NOT NULL DEFAULT 0;
