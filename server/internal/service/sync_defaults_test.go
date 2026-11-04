package service

// U6：集合级公共参数（项目级单例、服务端权威）的同步语义。

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"apidoc/internal/model"
)

func TestSyncProjectDefaultsDelivered(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	const doc = `{"headers":[{"name":"X-Common","value":"1","enabled":true}],"auth":{"type":"bearer","token":"t"}}`
	proj, err := f.s.repo.ProjectByID(ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("ProjectByID: %v", err)
	}
	proj.Defaults = doc
	if err := f.s.repo.UpdateProject(ctx, proj); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	// ① 增量响应必须附带 defaults（它不在 change_log 的 api/doc 流水里）。
	ch, err := f.s.SyncChanges(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, 200)
	if err != nil {
		t.Fatalf("SyncChanges: %v", err)
	}
	got := findDefaults(ch.Items)
	if got == "" {
		t.Fatalf("增量响应未附带 defaults: %+v", ch.Items)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("defaults 不是合法 JSON: %q", got)
	}
	if !strings.Contains(got, "X-Common") {
		t.Fatalf("defaults 内容不符: %q", got)
	}

	// ② 全量快照同样附带（首次拉取即可拿到）。
	sn, err := f.s.SyncSnapshot(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, 500)
	if err != nil {
		t.Fatalf("SyncSnapshot: %v", err)
	}
	if findDefaults(sn.Items) == "" {
		t.Fatalf("全量快照未附带 defaults: %+v", sn.Items)
	}

	// ③ 带 defaults 的推送（Web/其它端）按原文落库。
	newDoc := `{"headers":[{"name":"X-New","value":"2","enabled":true}]}`
	body, _ := json.Marshal(SyncPayload{Defaults: newDoc})
	out, err := f.s.SyncPush(ctx, f.admin.ID, f.admin.Type, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev-test",
		Ops: []SyncOpIn{{
			OpID: "op-defaults-1", Op: model.SyncOpUpsert, Type: model.SyncTypeDefaults,
			UID: fmt.Sprintf("project-%d", f.proj.ID), Payload: body,
		}},
	})
	if err != nil {
		t.Fatalf("SyncPush defaults: %v", err)
	}
	t.Logf("push 结果: %+v", out.Results)
	after, err := f.s.repo.ProjectByID(ctx, f.proj.ID)
	if err != nil {
		t.Fatalf("ProjectByID: %v", err)
	}
	if after.Defaults != newDoc {
		t.Fatalf("defaults 未落库: %q", after.Defaults)
	}
}

// findDefaults 从同步条目里取出 defaults 原文（没有返回空串）。
func findDefaults(items []SyncItem) string {
	for _, it := range items {
		if it.Type != model.SyncTypeDefaults {
			continue
		}
		var p SyncPayload
		if err := json.Unmarshal(it.Payload, &p); err == nil {
			return p.Defaults
		}
	}
	return ""
}

// U6：Web 侧的公共参数读写接口（服务端权威来源）。
func TestProjectDefaultsAPI(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	doc := `{"headers":[{"name":"X-Token","value":"t","enabled":true}]}`
	if err := f.s.SetProjectDefaults(ctx, f.admin.ID, f.admin.Type, f.proj.ID, doc); err != nil {
		t.Fatalf("SetProjectDefaults: %v", err)
	}
	got, err := f.s.GetProjectDefaults(ctx, f.admin.ID, f.admin.Type, f.proj.ID)
	if err != nil {
		t.Fatalf("GetProjectDefaults: %v", err)
	}
	if got != doc {
		t.Fatalf("读写不一致: %q", got)
	}

	// 非法 JSON 必须拒绝：存进去的坏数据会让客户端解析失败、公共头静默失效。
	if err := f.s.SetProjectDefaults(ctx, f.admin.ID, f.admin.Type, f.proj.ID, "{oops"); err == nil {
		t.Fatal("非法 JSON 应报错")
	}

	// 空串 = 清空。
	if err := f.s.SetProjectDefaults(ctx, f.admin.ID, f.admin.Type, f.proj.ID, ""); err != nil {
		t.Fatalf("清空: %v", err)
	}
	got, _ = f.s.GetProjectDefaults(ctx, f.admin.ID, f.admin.Type, f.proj.ID)
	if got != "" {
		t.Fatalf("清空后应为空串，实际 %q", got)
	}
}
