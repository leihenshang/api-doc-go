-- 0023_project_defaults：项目新增「集合级公共参数」（公共请求头/认证）。
-- defaults：JSON 文本，形如 {"headers":[{"name","value","enabled"}],"auth":{"type","token",...}}；
-- 服务端是唯一权威（客户端只读镜像，发送时在「请求未覆盖」前提下套用），因此不做结构校验之外的解释。
-- 等价 SQLite 版 migrations/sqlite/0023_project_defaults.up.sql。
ALTER TABLE "project" ADD COLUMN "defaults" text NOT NULL DEFAULT '';
