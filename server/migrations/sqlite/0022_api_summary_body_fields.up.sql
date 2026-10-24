-- 0022_api_summary_body_fields：接口新增「概述」与「请求体字段表格」两列（请求体字段表格与概述需求）。
-- http_request_summary：API 概述（纯文本一句话），对应同步 extra 键 summary。
-- http_request_body_fields：请求体字段表格（[BodyField] 的 JSON 数组字符串），对应同步 extra 键 bodyFields。
-- 服务端不解释语义、原样回传客户端；键集与客户端 collection/sync_extra_fields.go 逐字一致。
-- 等价 PostgreSQL 版 migrations/postgres/0022_api_summary_body_fields.up.sql。
ALTER TABLE `api` ADD COLUMN `http_request_summary` text NOT NULL DEFAULT '';
ALTER TABLE `api` ADD COLUMN `http_request_body_fields` text NOT NULL DEFAULT '';
