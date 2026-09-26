-- 0009 回滚：移除用户界面语言偏好字段。
ALTER TABLE `user_info` DROP COLUMN `lang`;
