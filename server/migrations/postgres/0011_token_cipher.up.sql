-- 0011_token_cipher：PAT 明文 AES-256-GCM 加密存储（S15），供前端「眼睛」回显明文。
-- 与 SQLite 版 migrations/sqlite/0011_token_cipher.up.sql 等价。
ALTER TABLE "user_token" ADD COLUMN "token_cipher" varchar(512) NOT NULL DEFAULT '';
