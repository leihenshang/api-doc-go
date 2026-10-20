package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

// pushAPI 构造一条 api upsert 并提交，返回服务端结果。opID 必须唯一（服务端按 op_id 幂等）。
func pushAPI(t *testing.T, s *Service, userID uint64, projectID uint64, uid, opID string, p SyncPayload) SyncOpOut {
	t.Helper()
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	// 超管类型直过权限校验（本用例验证同步语义，不验证权限矩阵）
	resp, err := s.SyncPush(context.Background(), userID, model.UserTypeSuperuser, &SyncPushReq{
		ProjectID:  projectID,
		DeviceID:   "dev-test",
		DeviceName: "测试设备",
		Ops:        []SyncOpIn{{OpID: opID, Op: model.SyncOpUpsert, Type: model.SyncTypeAPI, UID: uid, Payload: body}},
	})
	if err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Status != "ok" {
		t.Fatalf("push 结果异常: %+v", resp.Results)
	}
	return resp.Results[0]
}

// TestSyncPushAPIParams 客户端推送的请求头/请求参数/返回字段必须落到 api 表，
// 且能经增量载荷（loadPayload）原样读出（修复「同步后服务端看不到请求参数」）。
func TestSyncPushAPIParams(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()

	const (
		uid     = "api-params-uid-1"
		headers = `[{"name":"X-Token","value":"abc","description":""}]`
		params  = `[{"fieldName":"page","type":"string","required":false,"description":"页码","example":"1"},` +
			`{"fieldName":"name","type":"string","required":false,"description":"","example":"alice"}]`
		retParams = `[{"fieldName":"code","type":"integer","required":true,"description":"","example":"200"}]`
		retSample = `{"code":200}`
	)
	extra, _ := json.Marshal(map[string]any{
		"headers":   headers,
		"params":    params,
		"retType":   "object",
		"retParams": retParams,
		"retSample": retSample,
		"desc":      "接口说明",
		"protocol":  "http",
	})
	pushAPI(t, f.s, f.admin.ID, f.proj.ID, uid, "op-params-1", SyncPayload{
		Name: "同步接口", Method: "POST", URL: "/sync/users", Extra: string(extra),
	})

	a, err := f.s.repo.FindApiByExtUID(ctx, f.proj.ID, uid)
	if err != nil || a == nil {
		t.Fatalf("推送后应能按 ext_uid 查到接口: %v", err)
	}
	if a.HttpRequestHeader != headers || a.HttpRequestParams != params {
		t.Fatalf("请求头/参数未落库:\n header=%s\n params=%s", a.HttpRequestHeader, a.HttpRequestParams)
	}
	if a.HttpReturnType != "object" || a.HttpReturnParams != retParams ||
		a.HttpReturnSample != retSample || a.Description != "接口说明" || a.ProtocolType != "http" {
		t.Fatalf("返回字段/说明/协议未落库: %+v", a)
	}

	// 增量载荷：extra 必须是 JSON 对象且各键为 DB 原文（snapshot 同口径）。
	raw, err := f.s.loadPayload(ctx, f.proj.ID, model.SyncTypeAPI, uid)
	if err != nil {
		t.Fatalf("loadPayload: %v", err)
	}
	var got SyncPayload
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("载荷反序列化: %v", err)
	}
	var em map[string]any
	if err := json.Unmarshal([]byte(got.Extra), &em); err != nil {
		t.Fatalf("extra 应为 JSON 对象（旧版 snapshot 曾误塞裸文本）: %v", err)
	}
	if em["params"] != params || em["headers"] != headers || em["retParams"] != retParams {
		t.Fatalf("extra 回读不一致: %s", got.Extra)
	}
}

// TestSyncPushAPIExtendedFields 覆盖客户端富字段（S16 扩展）的落库与回传：
// 请求体/认证/请求设置/脚本/变量/断言/响应内容类型均以原始 JSON/文本列存储，增量载荷原样回传。
func TestSyncPushAPIExtendedFields(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()
	const uid = "api-ext-uid-1"
	const (
		bodyType  = "json"
		bodyVal   = `{"a":1}`
		bodyForm  = `[{"name":"k","value":"v","enabled":true}]`
		authVal   = `{"type":"bearer","token":"t"}`
		settings  = `{"timeout":10,"followRedirects":-1,"maxRedirects":0,"insecureSsl":1,"encodeUrl":true}`
		scriptVal = `{"preRequest":"p","postResponse":"q"}`
		varsVal   = `[{"name":"x","value":"1","enabled":true}]`
		assertVal = `[{"name":"eq","expr":"a==b"}]`
		retCT     = "xml"
	)
	extra, _ := json.Marshal(map[string]any{
		"bodyType": bodyType, "body": bodyVal, "bodyForm": bodyForm,
		"auth": authVal, "settings": settings, "script": scriptVal,
		"vars": varsVal, "asserts": assertVal, "retContentType": retCT,
	})
	pushAPI(t, f.s, f.admin.ID, f.proj.ID, uid, "op-ext-1", SyncPayload{Name: "ext", Method: "GET", URL: "/e", Extra: string(extra)})

	a, err := f.s.repo.FindApiByExtUID(ctx, f.proj.ID, uid)
	if err != nil || a == nil {
		t.Fatalf("推送后应能查到接口: %v", err)
	}
	if a.HttpRequestBodyType != bodyType || a.HttpRequestBody != bodyVal || a.HttpRequestBodyForm != bodyForm ||
		a.HttpAuth != authVal || a.RequestSettings != settings || a.HttpScript != scriptVal ||
		a.ScriptVars != varsVal || a.ScriptAsserts != assertVal || a.HttpReturnContentType != retCT {
		t.Fatalf("扩展字段未落库: %+v", a)
	}

	// 增量载荷必须原样回传这些键（与 applyApiExtra 键集对齐），供客户端双向同步。
	raw, err := f.s.loadPayload(ctx, f.proj.ID, model.SyncTypeAPI, uid)
	if err != nil {
		t.Fatalf("loadPayload: %v", err)
	}
	var got SyncPayload
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("载荷反序列化: %v", err)
	}
	var em map[string]any
	if err := json.Unmarshal([]byte(got.Extra), &em); err != nil {
		t.Fatalf("extra 应为 JSON 对象: %v", err)
	}
	for k, want := range map[string]string{
		"bodyType": bodyType, "body": bodyVal, "bodyForm": bodyForm, "auth": authVal,
		"settings": settings, "script": scriptVal, "vars": varsVal, "asserts": assertVal,
		"retContentType": retCT,
	} {
		if em[k] != want {
			t.Fatalf("回传 %s 不符：got %v want %s", k, em[k], want)
		}
	}
}

// TestSyncPushAPIEmptyExtraClears 空串键表示清空；缺键保持不动（整份覆盖 + 部分更新语义）。
func TestSyncPushAPIEmptyExtraClears(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()
	const uid = "api-params-uid-2"
	extra1, _ := json.Marshal(map[string]any{
		"headers": `[{"name":"H","value":"v","description":""}]`,
		"params":  `[{"fieldName":"q","type":"string","required":false,"description":"","example":"1"}]`,
		"desc":    "d",
	})
	pushAPI(t, f.s, f.admin.ID, f.proj.ID, uid, "op-clear-1", SyncPayload{
		Name: "x", Method: "GET", URL: "/x", Extra: string(extra1),
	})

	// 第二次推送只带空 headers（清空），不带 params（保留）。
	extra2, _ := json.Marshal(map[string]any{"headers": ""})
	pushAPI(t, f.s, f.admin.ID, f.proj.ID, uid, "op-clear-2", SyncPayload{
		Name: "x", Method: "GET", URL: "/x", Extra: string(extra2),
	})
	a, _ := f.s.repo.FindApiByExtUID(ctx, f.proj.ID, uid)
	if a == nil {
		t.Fatal("接口丢失")
	}
	if a.HttpRequestHeader != "" {
		t.Fatalf("空串应清空请求头, got %q", a.HttpRequestHeader)
	}
	if !strings.Contains(a.HttpRequestParams, `"q"`) {
		t.Fatalf("缺键参数应保留, got %q", a.HttpRequestParams)
	}
}

// TestSyncSnapshotAPIExtra 全量快照里 api 的 extra 必须是聚合 JSON（回归：曾误为 header 裸文本）。
func TestSyncSnapshotAPIExtra(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	const uid = "api-params-uid-3"
	extra, _ := json.Marshal(map[string]any{
		"headers": `[{"name":"H","value":"v","description":""}]`,
		"params":  `[{"fieldName":"q","type":"string","required":false,"description":"","example":"1"}]`,
	})
	pushAPI(t, f.s, f.admin.ID, f.proj.ID, uid, "op-snap-1", SyncPayload{
		Name: "快照接口", Method: "GET", URL: "/snap", Extra: string(extra),
	})

	sn, err := f.s.SyncSnapshot(context.Background(), 0, model.UserTypeSuperuser, f.proj.ID, 0, 200)
	if err != nil {
		t.Fatalf("SyncSnapshot: %v", err)
	}
	var found bool
	for _, it := range sn.Items {
		if it.Type != model.SyncTypeAPI || it.UID != uid {
			continue
		}
		found = true
		var p SyncPayload
		if err := json.Unmarshal(it.Payload, &p); err != nil {
			t.Fatalf("快照载荷反序列化: %v", err)
		}
		var em map[string]any
		if err := json.Unmarshal([]byte(p.Extra), &em); err != nil {
			t.Fatalf("快照 extra 必须是 JSON 对象, got %q", p.Extra)
		}
		if _, ok := em["params"].(string); !ok {
			t.Fatalf("快照 extra 缺少 params: %s", p.Extra)
		}
	}
	if !found {
		t.Fatal("快照中未找到推送的 api")
	}
}

// pushOp 通用 push 辅助：任意类型/操作一条，返回结果。
func pushOp(t *testing.T, s *Service, userID, projectID uint64, opID, op, typ, uid string, p SyncPayload) SyncOpOut {
	t.Helper()
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	// 超管类型直过权限校验（本用例验证同步语义，不验证权限矩阵）
	resp, err := s.SyncPush(context.Background(), userID, model.UserTypeSuperuser, &SyncPushReq{
		ProjectID: projectID, DeviceID: "dev-test", DeviceName: "测试设备",
		Ops: []SyncOpIn{{OpID: opID, Op: op, Type: typ, UID: uid, Payload: body}},
	})
	if err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("push 结果异常: %+v", resp.Results)
	}
	return resp.Results[0]
}

// TestSyncPushRequiresProjectPermission push 路径权限位校验（§4.2）：
// 非成员 / 只读成员被拒；授予 update 位后放行。堵住「任意持令牌者可推任意项目」。
func TestSyncPushRequiresProjectPermission(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx
	bob, err := f.s.Register(ctx, dto.RegisterReq{Name: "bob", Pwd: "p123456", RePwd: "p123456"})
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	body, _ := json.Marshal(SyncPayload{Name: "x", Method: "GET", URL: "/x"})
	pushAs := func(opID string) error {
		_, err := f.s.SyncPush(ctx, bob.ID, bob.Type, &SyncPushReq{
			ProjectID: f.proj.ID, DeviceID: "dev",
			Ops: []SyncOpIn{{OpID: opID, Op: model.SyncOpUpsert, Type: model.SyncTypeAPI, UID: "perm-uid", Payload: body}},
		})
		return err
	}
	if err := pushAs("op-perm-nonmember"); err == nil {
		t.Fatal("非成员 push 应被拒绝")
	}
	// 只读成员：无 update 位，仍应被拒
	if err := f.s.AddMember(ctx, f.admin.ID, f.admin.Type, dto.MemberAddReq{ProjectID: f.proj.ID, UserID: bob.ID, Permission: model.PermReadonly}); err != nil {
		t.Fatalf("add bob: %v", err)
	}
	if err := pushAs("op-perm-readonly"); err == nil {
		t.Fatal("只读成员 push 应被拒绝")
	}
	// 授予 update 后可推
	if err := f.s.SetPermission(ctx, f.admin.ID, f.admin.Type, dto.SetPermissionReq{ProjectID: f.proj.ID, UserID: bob.ID, Permission: model.PermUpdate}); err != nil {
		t.Fatalf("set bob update: %v", err)
	}
	if err := pushAs("op-perm-update"); err != nil {
		t.Fatalf("有 update 位成员 push 应成功: %v", err)
	}
}

// TestSyncVariableRoundTrip 变量类同步（S16）：环境/变量 push 落库 → 载荷回读 →
// enc:v1: 密文原样存储且不进解析表 → delete 软删生效。
func TestSyncVariableRoundTrip(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := f.ctx

	envUID, varUID := "env-sync-1", "var-sync-1"
	pushOp(t, f.s, f.admin.ID, f.proj.ID, "op-env-1", model.SyncOpUpsert, model.SyncTypeVarEnv, envUID,
		SyncPayload{Name: "qa"})
	ev, err := f.s.repo.FindVarEnvByExtUID(ctx, f.proj.ID, envUID)
	if err != nil || ev == nil || ev.Name != "qa" {
		t.Fatalf("环境未落库: %v %+v", err, ev)
	}

	enabled := true
	secret := "enc:v1:QUJD" // 模拟客户端密文（前缀即契约）
	pushOp(t, f.s, f.admin.ID, f.proj.ID, "op-var-1", model.SyncOpUpsert, model.SyncTypeVariable, varUID,
		SyncPayload{Name: "token", Value: secret, Secret: true, Enabled: &enabled, EnvUID: envUID})
	v, err := f.s.repo.FindVariableByExtUID(ctx, f.proj.ID, varUID)
	if err != nil || v == nil {
		t.Fatalf("变量未落库: %v", err)
	}
	if v.Value != secret || v.IsSecret != model.VarSecret || v.EnvID != ev.ID {
		t.Fatalf("变量落库不符: %+v", v)
	}

	// 载荷回读（增量与快照共用 loadPayload）
	raw, err := f.s.loadPayload(ctx, f.proj.ID, model.SyncTypeVariable, varUID)
	if err != nil {
		t.Fatalf("loadPayload: %v", err)
	}
	var got SyncPayload
	if err := json.Unmarshal(raw, &got); err != nil || got.Name != "token" || got.Value != secret ||
		!got.Secret || got.EnvUID != envUID {
		t.Fatalf("回传载荷不符: %s", raw)
	}

	// 解析表跳过密文：{{token}} 不会被服务端渲染替换（true = 保留历史含明文 secret 的语义）
	table, err := f.s.resolveTable(ctx, f.proj.ID, 0, ev.ID, true)
	if err != nil {
		t.Fatalf("resolveTable: %v", err)
	}
	if _, ok := table["token"]; ok {
		t.Fatal("加密变量不应进入解析表")
	}
	if out := renderVars("{{token}}", table); out != "{{token}}" {
		t.Fatalf("密文变量占位符应原样保留，got %q", out)
	}

	// delete：change_log 记 tombstone；实体与 DeleteApi 同口径直接删除、行不再可见
	pushOp(t, f.s, f.admin.ID, f.proj.ID, "op-var-del", model.SyncOpDelete, model.SyncTypeVariable, varUID, SyncPayload{})
	v2, err := f.s.repo.FindVariableByExtUID(ctx, f.proj.ID, varUID)
	if err != nil || v2 != nil {
		t.Fatalf("变量删除后应不可见: %v %+v", err, v2)
	}
}

// TestSyncExampleRoundTrip 响应示例：push 落库 → 增量/快照载荷可回读 → delete 生效。
func TestSyncExampleRoundTrip(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()

	// 先有宿主接口（示例挂在接口上）
	const apiUID = "ex-host-api"
	pushAPI(t, f.s, f.admin.ID, f.proj.ID, apiUID, "op-ex-api", SyncPayload{
		Name: "宿主接口", Method: "GET", URL: "/ex",
	})
	const uid = "ex-sample-1"
	if out := pushOp(t, f.s, f.admin.ID, f.proj.ID, "op-ex-1", model.SyncOpUpsert, model.SyncTypeExample, uid, SyncPayload{
		Name: "成功", APIUID: apiUID, Sample: `{"code":200}`, Status: 200, ContentType: "application/json", Seq: 1,
	}); out.Status != "ok" {
		t.Fatalf("example push 应成功: %+v", out)
	}

	e, err := f.s.repo.FindApiReturnSampleByExtUID(ctx, f.proj.ID, uid)
	if err != nil || e == nil {
		t.Fatalf("推送后应能查到示例: %v", err)
	}
	if e.Name != "成功" || e.SampleText != `{"code":200}` || e.Status != 200 {
		t.Fatalf("示例字段不符: %+v", e)
	}

	// 增量载荷回读：api_uid 必须还原为宿主接口的 ext_uid
	raw, err := f.s.loadPayload(ctx, f.proj.ID, model.SyncTypeExample, uid)
	if err != nil {
		t.Fatalf("loadPayload(example): %v", err)
	}
	var p SyncPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("载荷反序列化: %v", err)
	}
	if p.APIUID != apiUID || p.Name != "成功" || p.Sample != `{"code":200}` {
		t.Fatalf("载荷回读不符: %+v", p)
	}

	// 快照包含示例
	sn, err := f.s.SyncSnapshot(ctx, 0, model.UserTypeSuperuser, f.proj.ID, 0, 200)
	if err != nil {
		t.Fatalf("SyncSnapshot: %v", err)
	}
	inSnap := false
	for _, it := range sn.Items {
		if it.Type == model.SyncTypeExample && it.UID == uid {
			inSnap = true
		}
	}
	if !inSnap {
		t.Fatal("快照中未找到示例")
	}

	// 删除传播
	if out := pushOp(t, f.s, f.admin.ID, f.proj.ID, "op-ex-2", model.SyncOpDelete, model.SyncTypeExample, uid, SyncPayload{}); out.Status != "ok" {
		t.Fatalf("example delete 应成功: %+v", out)
	}
	gone, err := f.s.repo.FindApiReturnSampleByExtUID(ctx, f.proj.ID, uid)
	if err != nil || gone != nil {
		t.Fatalf("删除后不应再查到示例: %v %v", gone, err)
	}

	// 缺宿主的示例被拒绝
	if out := pushOp(t, f.s, f.admin.ID, f.proj.ID, "op-ex-3", model.SyncOpUpsert, model.SyncTypeExample, "ex-orphan", SyncPayload{
		Name: "孤儿", APIUID: "no-such-api",
	}); out.Status != "rejected" {
		t.Fatalf("缺少宿主接口应拒绝: %+v", out)
	}
}
