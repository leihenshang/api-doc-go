-- 0006 回滚：移除备忘置顶字段。
ALTER TABLE `bug` DROP COLUMN `is_top`;
