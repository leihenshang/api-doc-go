-- 0021_content_snapshot：接口/文档保存快照（RM-05）。
-- 每次 Web 保存后写入一份完整载荷，用于字段级 diff 与回滚；按实体+revision 倒序查询。
CREATE TABLE IF NOT EXISTS "content_snapshot" (
    "id" BIGSERIAL PRIMARY KEY,
    "project_id" BIGINT NOT NULL,
    "entity_type" VARCHAR(20) NOT NULL,
    "entity_id" BIGINT NOT NULL,
    "revision" BIGINT NOT NULL DEFAULT 0,
    "actor_id" BIGINT NOT NULL DEFAULT 0,
    "action" VARCHAR(20) NOT NULL DEFAULT 'update',
    "payload" TEXT NOT NULL,
    "create_time" TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS "idx_snapshot_project" ON "content_snapshot"("project_id");
CREATE INDEX IF NOT EXISTS "idx_snapshot_entity" ON "content_snapshot"("entity_type", "entity_id");
