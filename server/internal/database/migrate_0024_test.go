package database

// 0024_group_unify_type：接口与文档共用一套目录的迁移行为。
//
// 这里不跑完整 Migrate（那需要整套表），只把 0024 的脚本作用在最小的
// group/doc 表上，断言「同名同层合并 + 剩余 type 统一」这两件事真的发生了。

import (
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigration0024UnifiesGroupTypes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:0024test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE "group" (id integer primary key, p_id integer, project_id integer, title text, priority integer, type integer, is_deleted integer default 0)`,
		`CREATE TABLE doc (id integer primary key, group_id integer, project_id integer, title text, is_deleted integer default 0)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	// 同层重名：接口分组（type=1）与文档分组（type=3）都叫「公共」
	// 另有一个只存在于 type=3 的「草稿」分组，迁移后应保留但改成 type=1。
	sql := []string{
		`INSERT INTO "group"(id,p_id,project_id,title,priority,type) VALUES (1,0,7,'公共',0,1)`,
		`INSERT INTO "group"(id,p_id,project_id,title,priority,type) VALUES (2,0,7,'公共',0,3)`,
		`INSERT INTO "group"(id,p_id,project_id,title,priority,type) VALUES (3,0,7,'草稿',0,3)`,
		`INSERT INTO "group"(id,p_id,project_id,title,priority,type) VALUES (4,2,7,'子目录',0,3)`,
		`INSERT INTO doc(id,group_id,project_id,title) VALUES (10,2,7,'挂在文档分组上')`,
		`INSERT INTO doc(id,group_id,project_id,title) VALUES (11,4,7,'挂在子目录上')`,
	}
	for _, s := range sql {
		if err := db.Exec(s).Error; err != nil {
			t.Fatalf("seed: %v", s)
		}
	}

	script, err := os.ReadFile("../../migrations/sqlite/0024_group_unify_type.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if err := execScript(db, string(script)); err != nil {
		t.Fatalf("apply 0024: %v", err)
	}

	// ① 挂在与接口分组重名的文档分组上的文档，应改挂到 type=1 的那个 (id=1)
	var gid uint64
	if err := db.Raw(`SELECT group_id FROM doc WHERE id = 10`).Scan(&gid).Error; err != nil {
		t.Fatalf("query doc: %v", err)
	}
	if gid != 1 {
		t.Fatalf("文档应改挂到同名接口分组 id=1，实际 %d", gid)
	}

	// ② 重名的 type=3 壳被删除，只留 type=1 的那个
	var dup int64
	if err := db.Raw(`SELECT count(*) FROM "group" WHERE id = 2`).Scan(&dup).Error; err != nil {
		t.Fatalf("query dup: %v", err)
	}
	if dup != 0 {
		t.Fatalf("同名 type=3 分组应被合并删除")
	}

	// ③ 子目录（原来挂在被合并的 id=2 下）改挂到 id=1
	var pid uint64
	if err := db.Raw(`SELECT p_id FROM "group" WHERE id = 4`).Scan(&pid).Error; err != nil {
		t.Fatalf("query child: %v", err)
	}
	if pid != 1 {
		t.Fatalf("子目录应改挂到 id=1，实际 %d", pid)
	}

	// ④ 没有同名接口分组的「草稿」保留，但类型统一成 1
	var typ int8
	if err := db.Raw(`SELECT type FROM "group" WHERE id = 3`).Scan(&typ).Error; err != nil {
		t.Fatalf("query draft: %v", err)
	}
	if typ != 1 {
		t.Fatalf("剩余文档分组类型应统一为 1，实际 %d", typ)
	}

	// ⑤ 全表不应再有 type=3
	var left int64
	if err := db.Raw(`SELECT count(*) FROM "group" WHERE type = 3`).Scan(&left).Error; err != nil {
		t.Fatalf("query left: %v", err)
	}
	if left != 0 {
		t.Fatalf("迁移后不应残留 type=3，got %d", left)
	}
}
