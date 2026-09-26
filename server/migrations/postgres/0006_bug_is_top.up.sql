-- 0006_bug_is_top：备忘清单支持置顶（列表按 is_top 优先排序）。
-- 与 SQLite 版 migrations/sqlite/0006_bug_is_top.up.sql 等价。
ALTER TABLE "bug" ADD COLUMN "is_top" smallint NOT NULL DEFAULT 0;
