-- 0024_group_unify_type：接口与文档共用一套目录。
--
-- 背景：客户端同步上来的分组一律是 type=1（同步协议里 group 不带 type），
-- 而 Web「项目文档」页历史上建的分组是 type=3。合并成一个「文档」工作台后，
-- 同一层会出现两个同名文件夹（接口的「用户」+ 文档的「用户」）。
--
-- 处理：① type=3 的文档/子分组并入同层同名的 type=1 分组；② 删掉已空的同名壳；
-- ③ 剩余 type=3 统一改成 type=1。层级深时 ①② 会逐层生效（自底向上一次迁移即可，
-- 父级先换 id 后子级仍能按新 p_id 匹配上）。
--
-- 等价 SQLite 版 migrations/sqlite/0024_group_unify_type.up.sql。

-- ① 文档改挂到同层同名的接口分组
UPDATE doc SET group_id = (
  SELECT t.id FROM "group" s, "group" t
  WHERE s.id = doc.group_id AND s.type = 3
    AND t.type = 1 AND t.project_id = s.project_id AND t.p_id = s.p_id AND t.title = s.title
  LIMIT 1
)
WHERE EXISTS (
  SELECT 1 FROM "group" s, "group" t
  WHERE s.id = doc.group_id AND s.type = 3
    AND t.type = 1 AND t.project_id = s.project_id AND t.p_id = s.p_id AND t.title = s.title
);

-- ② 子分组改挂到同层同名的接口分组下
UPDATE "group" SET p_id = (
  SELECT t.id FROM "group" s, "group" t
  WHERE s.id = "group".p_id AND s.type = 3
    AND t.type = 1 AND t.project_id = s.project_id AND t.p_id = s.p_id AND t.title = s.title
  LIMIT 1
)
WHERE EXISTS (
  SELECT 1 FROM "group" s, "group" t
  WHERE s.id = "group".p_id AND s.type = 3
    AND t.type = 1 AND t.project_id = s.project_id AND t.p_id = s.p_id AND t.title = s.title
);

-- ③ 删掉已被合并掉的空壳（此时其文档与子分组都迁走了）
DELETE FROM "group" AS g
WHERE g.type = 3 AND EXISTS (
  SELECT 1 FROM "group" t
  WHERE t.type = 1 AND t.project_id = g.project_id AND t.p_id = g.p_id AND t.title = g.title
);

-- ④ 剩下的 type=3 统一成接口分组类型
UPDATE "group" SET type = 1 WHERE type = 3;
