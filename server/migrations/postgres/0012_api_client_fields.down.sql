-- 0012 回滚：移除客户端富字段列。
ALTER TABLE "api" DROP COLUMN IF EXISTS "http_request_body_type";
ALTER TABLE "api" DROP COLUMN IF EXISTS "http_request_body";
ALTER TABLE "api" DROP COLUMN IF EXISTS "http_request_body_form";
ALTER TABLE "api" DROP COLUMN IF EXISTS "http_request_body_descs";
ALTER TABLE "api" DROP COLUMN IF EXISTS "http_auth";
ALTER TABLE "api" DROP COLUMN IF EXISTS "request_settings";
ALTER TABLE "api" DROP COLUMN IF EXISTS "http_script";
ALTER TABLE "api" DROP COLUMN IF EXISTS "script_vars";
ALTER TABLE "api" DROP COLUMN IF EXISTS "script_asserts";
ALTER TABLE "api" DROP COLUMN IF EXISTS "http_return_content_type";
