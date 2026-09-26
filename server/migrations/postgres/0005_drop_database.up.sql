-- 0005_drop_database：移除「数据字典（外部数据库连接）」功能，删除连接表。
-- 与 SQLite 版 migrations/sqlite/0005_drop_database.up.sql 等价。
DROP TABLE IF EXISTS "database";
