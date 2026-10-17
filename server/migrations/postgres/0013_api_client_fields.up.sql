-- 0013_api_client_fields：接口新增「客户端富字段」列（请求体/认证/请求设置/脚本/变量/断言/响应内容类型/请求体备注）。
-- 均以原始 JSON/文本列存储，服务端不解释语义、原样回传客户端；键集与客户端 collection/sync_extra_fields.go 逐字一致。
-- 等价 SQLite 版 migrations/sqlite/0013_api_client_fields.up.sql。
ALTER TABLE "api" ADD COLUMN "http_request_body_type" character varying(100) NOT NULL DEFAULT '';
ALTER TABLE "api" ADD COLUMN "http_request_body" text NOT NULL DEFAULT '';
ALTER TABLE "api" ADD COLUMN "http_request_body_form" text NOT NULL DEFAULT '';
ALTER TABLE "api" ADD COLUMN "http_request_body_descs" text NOT NULL DEFAULT '';
ALTER TABLE "api" ADD COLUMN "http_auth" text NOT NULL DEFAULT '';
ALTER TABLE "api" ADD COLUMN "request_settings" text NOT NULL DEFAULT '';
ALTER TABLE "api" ADD COLUMN "http_script" text NOT NULL DEFAULT '';
ALTER TABLE "api" ADD COLUMN "script_vars" text NOT NULL DEFAULT '';
ALTER TABLE "api" ADD COLUMN "script_asserts" text NOT NULL DEFAULT '';
ALTER TABLE "api" ADD COLUMN "http_return_content_type" character varying(100) NOT NULL DEFAULT '';
