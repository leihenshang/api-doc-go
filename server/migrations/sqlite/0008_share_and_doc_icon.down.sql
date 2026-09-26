-- 0008 回滚：移除分享链接表与文档图标字段。
DROP TABLE IF EXISTS `share_link`;
ALTER TABLE `doc` DROP COLUMN `icon`;
