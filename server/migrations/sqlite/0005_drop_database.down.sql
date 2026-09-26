-- 0005 回滚：重建连接表（结构同 0001_init）。
CREATE TABLE `database` (`id` integer PRIMARY KEY AUTOINCREMENT,`is_deleted` integer NOT NULL DEFAULT 0,`create_time` datetime,`update_time` datetime,`address` text NOT NULL,`port` integer NOT NULL DEFAULT 0,`username` text NOT NULL,`password` text NOT NULL,`database_name` text NOT NULL,`project_id` integer NOT NULL,`user_id` integer NOT NULL DEFAULT 0,`type` integer NOT NULL DEFAULT 1,`description` text);
CREATE INDEX `idx_database_project_id` ON `database`(`project_id`);
