-- 0021_content_snapshot：接口/文档保存快照（RM-05）。
-- 每次 Web 保存后写入一份完整载荷，用于字段级 diff 与回滚；按实体+revision 倒序查询。
CREATE TABLE IF NOT EXISTS content_snapshot (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id INTEGER NOT NULL,
    revision INTEGER NOT NULL DEFAULT 0,
    actor_id INTEGER NOT NULL DEFAULT 0,
    action TEXT NOT NULL DEFAULT 'update',
    payload TEXT NOT NULL,
    create_time DATETIME
);
CREATE INDEX IF NOT EXISTS idx_snapshot_project ON content_snapshot(project_id);
CREATE INDEX IF NOT EXISTS idx_snapshot_entity ON content_snapshot(entity_type, entity_id);
