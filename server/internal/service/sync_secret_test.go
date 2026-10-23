package service

// H-11 回归：同步载荷里的 secret 变量值。
//   - 客户端密文（enc:v1:）始终原样回传——服务端不解析，客户端可用口令解密后使用；
//   - 明文 secret 只对有明文权限（管理员/负责人/update 位）的调用方下发，否则 redacted=true；
//   - 下发明文时写一条「同步读取敏感变量明文」审计，只读用户不写。

import (
	"encoding/json"
	"strings"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/page"
)

// findVarPayload 从同步条目里按变量名找 payload；找不到直接失败。
func findVarPayload(t *testing.T, items []SyncItem, name string) SyncPayload {
	t.Helper()
	for _, it := range items {
		if it.Type != model.SyncTypeVariable || len(it.Payload) == 0 {
			continue
		}
		var p SyncPayload
		if err := json.Unmarshal(it.Payload, &p); err == nil && p.Name == name {
			return p
		}
	}
	t.Fatalf("同步条目里没有变量 %q", name)
	return SyncPayload{}
}

// countSecretAudit 统计项目操作日志中「同步读取敏感变量明文」且包含 name 的条数。
func countSecretAudit(t *testing.T, f *varFixture, name string) int {
	t.Helper()
	res, err := f.s.ListLogs(f.ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, model.LogObjectVariable,
		page.Query{PageNum: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListLogs: %v", err)
	}
	n := 0
	for _, lg := range res.Items {
		if strings.Contains(lg.Content, "同步读取敏感变量明文") && strings.Contains(lg.Content, name) {
			n++
		}
	}
	return n
}

// TestSyncSecretPayloadRedaction 校验只读成员拿不到明文、密文原样，管理员拿明文且留审计。
func TestSyncSecretPayloadRedaction(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	bob := f.registerReadonly(t, "bob-secret")
	ctx := f.ctx

	const plain, cipher = "sk-live-plain", "enc:v1:QUJD"
	f.newVar(t, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID,
		Name: "plainToken", Value: strPtr(plain), IsSecret: model.VarSecret,
	})
	f.newVar(t, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: f.proj.ID, EnvID: f.dev.ID,
		Name: "cipherToken", Value: strPtr(cipher), IsSecret: model.VarSecret,
	})

	// 只读成员：快照与增量都不下发明文；密文始终原样（客户端可解密使用）。
	snap, err := f.s.SyncSnapshot(ctx, bob.ID, bob.Type, f.proj.ID, 0, 500)
	if err != nil {
		t.Fatalf("SyncSnapshot(readonly): %v", err)
	}
	if p := findVarPayload(t, snap.Items, "plainToken"); p.Value != "" || !p.Redacted || !p.Secret {
		t.Fatalf("只读快照不应下发明文 secret: %+v", p)
	}
	if p := findVarPayload(t, snap.Items, "cipherToken"); p.Value != cipher || p.Redacted {
		t.Fatalf("密文应原样回传: %+v", p)
	}

	changes, err := f.s.SyncChanges(ctx, bob.ID, bob.Type, f.proj.ID, 0, 200)
	if err != nil {
		t.Fatalf("SyncChanges(readonly): %v", err)
	}
	if p := findVarPayload(t, changes.Items, "plainToken"); p.Value != "" || !p.Redacted {
		t.Fatalf("只读增量不应下发明文 secret: %+v", p)
	}
	if n := countSecretAudit(t, f, "plainToken"); n != 0 {
		t.Fatalf("只读用户不应产生明文审计，got %d", n)
	}

	// 管理员（有明文权限）：原样回传，并写一条审计。
	adminSnap, err := f.s.SyncSnapshot(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, 500)
	if err != nil {
		t.Fatalf("SyncSnapshot(admin): %v", err)
	}
	if p := findVarPayload(t, adminSnap.Items, "plainToken"); p.Value != plain || p.Redacted {
		t.Fatalf("管理员应拿到明文原样回传: %+v", p)
	}
	if n := countSecretAudit(t, f, "plainToken"); n != 1 {
		t.Fatalf("管理员同步明文应写 1 条审计，got %d", n)
	}
}
