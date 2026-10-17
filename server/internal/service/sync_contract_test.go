package service

// P2-1：同步线协议已下沉到 share/syncapi，服务端 DTO 只是它的别名。
// 这里守两件事：
//  1. 落库用的 entity_type / op 字符串必须与线上协议常量一致（不一致会导致
//     change_log 写入一种、客户端按另一种读取，表现为「某类条目永远同步不过去」）；
//  2. DTO 确实是共享类型的别名（而不是又抄了一份）。

import (
	"encoding/json"
	"reflect"
	"testing"

	"apidoc/internal/model"

	syncapi "github.com/leihenshang/api-doc-go-share/syncapi"
)

// TestSyncTypeConstantsMatchWire 落库字符串 == 线上协议字符串。
func TestSyncTypeConstantsMatchWire(t *testing.T) {
	t.Parallel()
	pairs := []struct {
		name     string
		db, wire string
	}{
		{"group", model.SyncTypeGroup, syncapi.TypeGroup},
		{"api", model.SyncTypeAPI, syncapi.TypeAPI},
		{"doc", model.SyncTypeDoc, syncapi.TypeDoc},
		{"example", model.SyncTypeExample, syncapi.TypeExample},
		{"variable", model.SyncTypeVariable, syncapi.TypeVariable},
		{"variable_env", model.SyncTypeVarEnv, syncapi.TypeVarEnv},
		{"upsert", model.SyncOpUpsert, syncapi.OpUpsert},
		{"delete", model.SyncOpDelete, syncapi.OpDelete},
	}
	for _, p := range pairs {
		if p.db != p.wire {
			t.Fatalf("%s: 落库值 %q ≠ 线协议值 %q（客户端会认不出这一类条目）", p.name, p.db, p.wire)
		}
	}
	if SyncProto != syncapi.Proto {
		t.Fatalf("服务端 SyncProto=%d 与 syncapi.Proto=%d 不一致", SyncProto, syncapi.Proto)
	}
}

// TestDTOsAreAliasesOfSharedTypes DTO 必须是共享类型的别名，不是本地副本。
func TestDTOsAreAliasesOfSharedTypes(t *testing.T) {
	t.Parallel()
	pairs := []struct {
		local, shared reflect.Type
	}{
		{reflect.TypeOf(SyncPayload{}), reflect.TypeOf(syncapi.Payload{})},
		{reflect.TypeOf(SyncItem{}), reflect.TypeOf(syncapi.Item{})},
		{reflect.TypeOf(SyncOpIn{}), reflect.TypeOf(syncapi.Op{})},
		{reflect.TypeOf(SyncOpOut{}), reflect.TypeOf(syncapi.OpResult{})},
		{reflect.TypeOf(SyncPushReq{}), reflect.TypeOf(syncapi.PushReq{})},
		{reflect.TypeOf(SyncPushResp{}), reflect.TypeOf(syncapi.PushResp{})},
		{reflect.TypeOf(SyncChangesResp{}), reflect.TypeOf(syncapi.Changes{})},
		{reflect.TypeOf(SyncSnapshotResp{}), reflect.TypeOf(syncapi.Snapshot{})},
		{reflect.TypeOf(SyncStateResp{}), reflect.TypeOf(syncapi.State{})},
		{reflect.TypeOf(MetaResp{}), reflect.TypeOf(syncapi.Meta{})},
	}
	for _, p := range pairs {
		if p.local != p.shared {
			t.Fatalf("%s 不是 syncapi 的别名（又抄了一份，字段会漂移）", p.local)
		}
	}
}

// TestEmptyPayloadEmitsNoKeys 空载荷不产生任何 JSON 键：所有字段都是 omitempty，
// 避免把「未设置」当成「设置为零值」推给对端。
func TestEmptyPayloadEmitsNoKeys(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(SyncPayload{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != "{}" {
		t.Fatalf("空载荷应为 {}，got %s", b)
	}
}
