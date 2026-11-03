package service

// R2：全局变量（scope=3，跨项目）必须随同步下发到客户端。
// 不下发的话，客户端解析 {{var}} 拿不到管理员配的全局变量 ——
// 同一请求在 Web 与客户端会跑出不同结果。

import (
	"encoding/json"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

func TestSyncGlobalVariablesAreDelivered(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	val := "https://api.example.com"
	if _, err := f.s.CreateVariable(ctx, f.admin.ID, f.admin.Type, dto.VariableReq{
		Scope: model.VarScopeGlobal, Name: "global_host", Value: &val,
	}); err != nil {
		t.Fatalf("CreateVariable 全局变量: %v", err)
	}

	// ① 增量响应必须附带全局变量。
	ch, err := f.s.SyncChanges(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, 200)
	if err != nil {
		t.Fatalf("SyncChanges: %v", err)
	}
	if !hasGlobalVar(ch.Items, "global_host") {
		t.Fatalf("增量响应未附带全局变量: %+v", ch.Items)
	}

	// ② 全量快照同样要附带（首次拉取即可拿到，不必等下一轮增量）。
	sn, err := f.s.SyncSnapshot(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, 500)
	if err != nil {
		t.Fatalf("SyncSnapshot: %v", err)
	}
	if !hasGlobalVar(sn.Items, "global_host") {
		t.Fatalf("全量快照未附带全局变量: %+v", sn.Items)
	}
}

// hasGlobalVar 判断条目里是否有指定名字的全局变量（载荷 scope=3）。
func hasGlobalVar(items []SyncItem, name string) bool {
	for _, it := range items {
		if it.Type != model.SyncTypeVariable || len(it.Payload) == 0 {
			continue
		}
		var p SyncPayload
		if err := json.Unmarshal(it.Payload, &p); err != nil {
			continue
		}
		if p.Scope == int(model.VarScopeGlobal) && p.Name == name {
			return true
		}
	}
	return false
}
