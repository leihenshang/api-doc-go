package service

// P2-26：SyncSnapshot 曾经有个 N+1 —— 每行都要单查一次「父分组 / 所属分组 / 所属接口 /
// 所属环境」的 ext_uid，200 行的快照就是 200+ 次数据库往返。
// 这里不测耗时（易抖动），直接数 SQL 条数：快照的查询次数必须是「按表数量级」，
// 而不是「按行数量级」。

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"apidoc/internal/config"
	"apidoc/internal/database"
	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// countingLogger 统计执行过的 SQL 条数（只关心条数，不关心内容）。
type countingLogger struct {
	mu    sync.Mutex
	count int
}

func (l *countingLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.mu.Lock()
	l.count++
	l.mu.Unlock()
}

func (l *countingLogger) LogMode(logger.LogLevel) logger.Interface { return l }
func (l *countingLogger) Info(context.Context, string, ...any)     {}
func (l *countingLogger) Warn(context.Context, string, ...any)     {}
func (l *countingLogger) Error(context.Context, string, ...any)    {}

func (l *countingLogger) reset() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := l.count
	l.count = 0
	return n
}

// TestSyncSnapshotQueryCountIsFlat 行数翻倍，查询条数不跟着涨。
func TestSyncSnapshotQueryCountIsFlat(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cl := &countingLogger{}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "q.db")),
		&gorm.Config{Logger: cl, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })

	cfg := &config.Config{}
	cfg.JWT.Secret = "q-test"
	s := New(repository.New(db, cfg), cfg)
	admin, err := s.InitAdmin(ctx, "admin", "admin123")
	if err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	// 1 个分组 + 40 个接口，全挂在同一分组下（最坏情况：每行都要反查分组 uid）
	p, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "工程量"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	g, err := s.CreateGroup(ctx, admin.ID, admin.Type, dto.GroupCreateReq{
		ProjectID: p.ID, Title: "用户", Type: model.GroupTypeAPI,
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	for i := 0; i < 40; i++ {
		if _, err := s.CreateApi(ctx, admin.ID, admin.Type, dto.ApiCreateReq{
			ProjectID: p.ID, GroupID: g.ID, ApiName: "接口" + itoaInt(i),
			HttpMethodType: "GET", URL: "/v1/query",
		}); err != nil {
			t.Fatalf("CreateApi #%d: %v", i, err)
		}
	}

	cl.reset()
	snap, err := s.SyncSnapshot(ctx, admin.ID, model.UserTypeSuperuser, p.ID, 0, 200)
	if err != nil {
		t.Fatalf("SyncSnapshot: %v", err)
	}
	if len(snap.Items) < 40 {
		t.Fatalf("快照应包含全部条目，got %d", len(snap.Items))
	}
	queries := cl.reset()
	// 6 张表各一次列表查询 + 游标 1 次 + 少量杂项。给到 20 已经很宽松；
	// 若回到「每行一次反查」，40 行这里会是 40+。
	if queries > 20 {
		t.Fatalf("快照查询条数应随表数增长而不是行数（40 行时实际 %d 条）", queries)
	}

	// 行数再翻一倍，查询条数不应增加
	for i := 40; i < 80; i++ {
		if _, err := s.CreateApi(ctx, admin.ID, admin.Type, dto.ApiCreateReq{
			ProjectID: p.ID, GroupID: g.ID, ApiName: "接口" + itoaInt(i),
			HttpMethodType: "GET", URL: "/v1/query",
		}); err != nil {
			t.Fatalf("CreateApi #%d: %v", i, err)
		}
	}
	cl.reset()
	if _, err := s.SyncSnapshot(ctx, admin.ID, model.UserTypeSuperuser, p.ID, 0, 200); err != nil {
		t.Fatalf("SyncSnapshot2: %v", err)
	}
	if queries2 := cl.reset(); queries2 > queries+2 {
		t.Fatalf("行数翻倍后查询条数不应增加：%d → %d", queries, queries2)
	}
}

func itoaInt(n int) string {
	b, _ := json.Marshal(n)
	return strings.TrimSpace(string(b))
}
