package service

// R2 补强：/sync/state 要回报全局变量的最新变更 id，
// 客户端据此在「项目游标没动、全局变量变了」时仍拉一次增量。

import (
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

func TestSyncStateGlobalCursorAdvances(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	before, err := f.s.SyncState(ctx, f.admin.ID, f.admin.Type, f.proj.ID)
	if err != nil {
		t.Fatalf("SyncState: %v", err)
	}

	val := "https://global.example.com"
	if _, err := f.s.CreateVariable(ctx, f.admin.ID, f.admin.Type, dto.VariableReq{
		Scope: model.VarScopeGlobal, Name: "g_state", Value: &val,
	}); err != nil {
		t.Fatalf("CreateVariable: %v", err)
	}

	after, err := f.s.SyncState(ctx, f.admin.ID, f.admin.Type, f.proj.ID)
	if err != nil {
		t.Fatalf("SyncState: %v", err)
	}
	if after.GlobalCursor <= before.GlobalCursor {
		t.Fatalf("全局变量变更应推进 GlobalCursor：%d → %d", before.GlobalCursor, after.GlobalCursor)
	}
	// 全局变量不属于项目：项目游标不该因此前进。
	if after.Cursor != before.Cursor {
		t.Fatalf("全局变量不应推进项目游标：%d → %d", before.Cursor, after.Cursor)
	}
}
