-- 0013 回滚：移除客户端富字段列。
ALTER TABLE `api` DROP COLUMN `http_request_body_type`;
ALTER TABLE `api` DROP COLUMN `http_request_body`;
ALTER TABLE `api` DROP COLUMN `http_request_body_form`;
ALTER TABLE `api` DROP COLUMN `http_request_body_descs`;
ALTER TABLE `api` DROP COLUMN `http_auth`;
ALTER TABLE `api` DROP COLUMN `request_settings`;
ALTER TABLE `api` DROP COLUMN `http_script`;
ALTER TABLE `api` DROP COLUMN `script_vars`;
ALTER TABLE `api` DROP COLUMN `script_asserts`;
ALTER TABLE `api` DROP COLUMN `http_return_content_type`;
