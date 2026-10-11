-- 0011_token_cipher：PAT 明文 AES-256-GCM 加密存储（S15），供前端「眼睛」回显明文。
-- 旧行 token_cipher 为空：无法回显，提示重新创建。
ALTER TABLE user_token ADD COLUMN token_cipher VARCHAR(512) NOT NULL DEFAULT '';
