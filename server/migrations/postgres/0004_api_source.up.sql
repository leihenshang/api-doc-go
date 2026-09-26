-- 0004_api_source：接口新增来源字段（openapi-import / bruno-import，空表示手工创建）；
-- description 同时放宽为 text，便于 Bruno/OpenAPI 导入时保存完整的接口文档（Markdown）。
-- 与 SQLite 版 migrations/sqlite/0004_api_source.up.sql 等价。
ALTER TABLE "api" ADD COLUMN "source" character varying(100) NOT NULL DEFAULT '';
ALTER TABLE "api" ALTER COLUMN "description" TYPE text;
