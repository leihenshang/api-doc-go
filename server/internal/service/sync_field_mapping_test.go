package service

import (
	"context"
	"encoding/json"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
)

// TestFieldMappingSyncRoundTrip 字段字典全链路：Web 新建 → 变更流水 + 快照可见；
// 客户端 push upsert → 落库并按 ext_uid 回读；push delete → 物理删除并写 delete 流水。
func TestFieldMappingSyncRoundTrip(t *testing.T) {
	t.Parallel()
	f := newVarFixture(t)
	ctx := context.Background()

	// 1) Web 新建：必须分配同步身份并写 upsert 流水。
	fm, err := f.s.CreateFieldMapping(ctx, f.admin.ID, f.admin.Type, dto.FieldMappingReq{
		ProjectID: f.proj.ID, Field: "data.list[].id", Type: "integer", Description: "主键",
	})
	if err != nil {
		t.Fatalf("CreateFieldMapping: %v", err)
	}
	if fm.ExtUID == "" {
		t.Fatal("Web 新建字段字典应分配 ext_uid（否则无法参与同步配对）")
	}

	// 2) 快照必须包含该条目（客户端全量拉取路径）。
	sn, err := f.s.SyncSnapshot(ctx, f.admin.ID, f.admin.Type, f.proj.ID, 0, 500)
	if err != nil {
		t.Fatalf("SyncSnapshot: %v", err)
	}
	found := false
	for _, it := range sn.Items {
		if it.Type == model.SyncTypeFieldMapping && it.UID == fm.ExtUID {
			found = true
			var p SyncPayload
			if err := json.Unmarshal(it.Payload, &p); err != nil {
				t.Fatalf("载荷反序列化: %v", err)
			}
			if p.Field != "data.list[].id" || p.FieldType != "integer" || p.Description != "主键" {
				t.Fatalf("快照载荷不符: %+v", p)
			}
		}
	}
	if !found {
		t.Fatalf("快照未包含新建的字段字典条目（共 %d 条）", len(sn.Items))
	}

	// 3) 客户端 push 新建：按 uid 落库，loadPayload 可回读。
	const uid = "fm-push-uid-1"
	body, _ := json.Marshal(SyncPayload{Field: "data.name", FieldType: "string", Description: "名称"})
	resp, err := f.s.SyncPush(ctx, f.admin.ID, f.admin.Type, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev-test", DeviceName: "测试设备",
		Ops: []SyncOpIn{{OpID: "op-fm-1", Op: model.SyncOpUpsert, Type: model.SyncTypeFieldMapping, UID: uid, Payload: body}},
	})
	if err != nil {
		t.Fatalf("SyncPush upsert: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Status != "ok" {
		t.Fatalf("push upsert 结果异常: %+v", resp.Results)
	}
	pushed, err := f.s.repo.FindFieldMappingByExtUID(ctx, f.proj.ID, uid)
	if err != nil || pushed == nil {
		t.Fatalf("push 后应能按 ext_uid 查到: %v", err)
	}
	if pushed.LastChangeID == 0 {
		t.Fatal("push 后 last_change_id 应回写（否则下一轮必冲突）")
	}
	raw, err := f.s.loadPayload(ctx, f.proj.ID, model.SyncTypeFieldMapping, uid)
	if err != nil {
		t.Fatalf("loadPayload: %v", err)
	}
	var got SyncPayload
	if err := json.Unmarshal(raw, &got); err != nil || got.Field != "data.name" || got.FieldType != "string" {
		t.Fatalf("增量载荷不一致: %s (err=%v)", raw, err)
	}

	// 4) 同名不同 uid 必须拒绝（project 内 field 唯一）。
	dup, _ := json.Marshal(SyncPayload{Field: "data.name", FieldType: "string"})
	dupResp, err := f.s.SyncPush(ctx, f.admin.ID, f.admin.Type, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev-test",
		Ops: []SyncOpIn{{OpID: "op-fm-dup", Op: model.SyncOpUpsert, Type: model.SyncTypeFieldMapping, UID: "fm-push-uid-2", Payload: dup}},
	})
	if err != nil {
		t.Fatalf("SyncPush duplicate: %v", err)
	}
	if len(dupResp.Results) != 1 || dupResp.Results[0].Status != "rejected" || dupResp.Results[0].Reason != "field_duplicate" {
		t.Fatalf("重复 field 应被拒绝: %+v", dupResp.Results)
	}

	// 5) push delete：物理删除并写变更流水。
	delResp, err := f.s.SyncPush(ctx, f.admin.ID, f.admin.Type, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev-test",
		Ops: []SyncOpIn{{OpID: "op-fm-del", Op: model.SyncOpDelete, Type: model.SyncTypeFieldMapping, UID: uid}},
	})
	if err != nil {
		t.Fatalf("SyncPush delete: %v", err)
	}
	if len(delResp.Results) != 1 || delResp.Results[0].Status != "ok" {
		t.Fatalf("push delete 结果异常: %+v", delResp.Results)
	}
	if gone, err := f.s.repo.FindFieldMappingByExtUID(ctx, f.proj.ID, uid); err != nil || gone != nil {
		t.Fatalf("delete 后应查不到（err=%v）: %+v", err, gone)
	}
}
