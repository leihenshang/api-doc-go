package service

import (
	"context"
	"encoding/json"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

// TestSyncPushBadPayloadRejected 覆盖 P0-3：payload 反序列化失败必须 rejected，
// 不能用零值覆盖存量数据。
func TestSyncPushBadPayloadRejected(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	// 先建一条接口，确保「坏 payload 落到已存在实体」的场景成立
	if _, err := f.s.CreateApi(context.Background(), f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, ApiName: "keep-me", HttpMethodType: "GET", URL: "/keep",
	}); err != nil {
		t.Fatalf("CreateApi: %v", err)
	}
	before, err := f.s.ListApis(context.Background(), f.admin.ID, f.admin.Type, dto.ApiListQuery{ProjectID: f.proj.ID})
	if err != nil || len(before.Items) == 0 {
		t.Fatalf("ListApis: %v", err)
	}

	resp, err := f.s.SyncPush(context.Background(), f.admin.ID, model.UserTypeSuperuser, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev-test", DeviceName: "测试设备",
		Ops: []SyncOpIn{{
			OpID: "op-bad-payload", Op: model.SyncOpUpsert, Type: model.SyncTypeAPI,
			UID: before.Items[0].ExtUID, Payload: json.RawMessage(`{"title":`), // 非法 JSON
		}},
	})
	if err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Status != "rejected" {
		t.Fatalf("坏 payload 应被 rejected，实际 %+v", resp.Results)
	}

	// 存量数据不能被清空
	after, err := f.s.ListApis(context.Background(), f.admin.ID, f.admin.Type, dto.ApiListQuery{ProjectID: f.proj.ID})
	if err != nil {
		t.Fatalf("ListApis after: %v", err)
	}
	if len(after.Items) == 0 || after.Items[0].ApiName == "" {
		t.Fatalf("坏 payload 不应清空存量数据，实际 %+v", after.Items)
	}
}

// TestVariableWriteLogsChange 覆盖 P0-7：Web 写变量必须落 change_log 并回写 last_change_id，
// 否则客户端增量拉取永远收不到这次变更。
func TestVariableWriteLogsChange(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()
	val := "v1"
	v, err := f.s.CreateVariable(ctx, f.admin.ID, f.admin.Type, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: f.proj.ID, Name: "kv", Value: &val,
	})
	if err != nil {
		t.Fatalf("CreateVariable: %v", err)
	}
	got, err := f.s.repo.GetVariable(ctx, v.ID)
	if err != nil || got == nil {
		t.Fatalf("GetVariable: %v", err)
	}
	if got.ExtUID == "" {
		t.Fatalf("变量应带 ext_uid（Web 新建也要分配，便于同步）")
	}
	if got.LastChangeID == 0 {
		t.Fatalf("变量 last_change_id 应被回写（P0-7），实际 0")
	}
	// 变更流水里应能找到该变量的 upsert
	latest, err := f.s.repo.LatestChangeID(ctx, f.proj.ID)
	if err != nil || latest == 0 {
		t.Fatalf("项目应有变更流水: %v (latest=%d)", err, latest)
	}
}
