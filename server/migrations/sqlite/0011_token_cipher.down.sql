-- 0011 回滚：移除 token_cipher 列（已加密的明文将无法找回）。
ALTER TABLE user_token DROP COLUMN token_cipher;
