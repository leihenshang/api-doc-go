-- 0022 回滚：移除概述与请求体字段表格两列。
ALTER TABLE "api" DROP COLUMN IF EXISTS "http_request_summary";
ALTER TABLE "api" DROP COLUMN IF EXISTS "http_request_body_fields";
