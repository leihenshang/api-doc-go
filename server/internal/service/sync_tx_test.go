package service

// P1-1：一条 push 的「实体写入 + 变更流水 + 修订号 + 幂等记录」必须在同一事务里。
// 之前这几步各自提交，任何一步失败都会留下不一致状态：
//   - 流水写失败 → 数据改了但客户端永远拉不到；
//   - 幂等记录写失败 → 重放时查不到历史，同一 op 被再执行一遍；
//   - 业务拒绝时已写入的实体没人撤销。

import (
	"context"
	"encoding/json"
	"testing"

	"apidoc/internal/model"
)

// opCount 统计某条 op 是否留下了幂等记录。
func hasSyncOp(t *testing.T, f *varFixture, opID string) bool {
	t.Helper()
	prev, err := f.s.repo.GetSyncOp(context.Background(), opID)
	if err != nil {
		t.Fatalf("GetSyncOp: %v", err)
	}
	return prev != nil
}

// changeCount 统计项目的变更流水条数。
func changeCount(t *testing.T, f *varFixture) int64 {
	t.Helper()
	var n int64
	if err := f.s.repo.DB().Model(&model.ChangeLog{}).Where("project_id = ?", f.proj.ID).Count(&n).Error; err != nil {
		t.Fatalf("count change_log: %v", err)
	}
	return n
}

// TestSyncPushRejectedLeavesNoTrace 业务性拒绝必须整体回滚：
// 既不写幂等记录（否则客户端重试永远拿 rejected），也不写变更流水。
func TestSyncPushRejectedLeavesNoTrace(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()
	before := changeCount(t, f)

	resp, err := f.s.SyncPush(ctx, f.admin.ID, model.UserTypeSuperuser, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev",
		Ops: []SyncOpIn{{
			OpID: "op-rejected", Op: model.SyncOpUpsert, Type: model.SyncTypeAPI,
			UID: "uid-rejected", Payload: json.RawMessage(`{"name":`), // 非法 JSON ⇒ rejected
		}},
	})
	if err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Status != "rejected" {
		t.Fatalf("应被 rejected，got %+v", resp.Results)
	}
	if hasSyncOp(t, f, "op-rejected") {
		t.Fatal("rejected 的 op 不应留下幂等记录（否则重试永远拿不到重新处理）")
	}
	if after := changeCount(t, f); after != before {
		t.Fatalf("rejected 不应写变更流水，before=%d after=%d", before, after)
	}
}

// TestSyncPushOkIsAtomicAndIdempotent 成功路径三项都要落库，且重放同一 op 不再产生新流水。
func TestSyncPushOkIsAtomicAndIdempotent(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()
	payload, err := json.Marshal(SyncPayload{Title: "同步分组"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev",
		Ops: []SyncOpIn{{
			OpID: "op-ok", Op: model.SyncOpUpsert, Type: model.SyncTypeGroup,
			UID: "uid-group-ok", Payload: payload,
		}},
	}
	resp, err := f.s.SyncPush(ctx, f.admin.ID, model.UserTypeSuperuser, req)
	if err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	if resp.Results[0].Status != "ok" || resp.Results[0].ItemRev == 0 {
		t.Fatalf("应成功并返回修订号，got %+v", resp.Results[0])
	}
	// 1) 实体
	g, err := f.s.repo.FindGroupByExtUID(ctx, f.proj.ID, "uid-group-ok")
	if err != nil || g == nil {
		t.Fatalf("分组未落库: %v", err)
	}
	// 2) 修订号回写
	if g.LastChangeID != resp.Results[0].ItemRev {
		t.Fatalf("last_change_id 应等于返回的 ItemRev，got %d vs %d", g.LastChangeID, resp.Results[0].ItemRev)
	}
	// 3) 幂等记录
	if !hasSyncOp(t, f, "op-ok") {
		t.Fatal("成功的 op 必须留下幂等记录")
	}

	// 重放：命中幂等，直接返回历史结果，不再产生新流水。
	rev := changeCount(t, f)
	resp2, err := f.s.SyncPush(ctx, f.admin.ID, model.UserTypeSuperuser, req)
	if err != nil {
		t.Fatalf("SyncPush(replay): %v", err)
	}
	if resp2.Results[0].Status != "ok" || resp2.Results[0].ItemRev != resp.Results[0].ItemRev {
		t.Fatalf("重放应返回同一结果，got %+v", resp2.Results[0])
	}
	if after := changeCount(t, f); after != rev {
		t.Fatalf("重放不应新增流水，before=%d after=%d", rev, after)
	}
}

// TestSyncPushConflictKeepsIdempotencyRecord 冲突要落幂等记录：
// 重复推送同一条过期数据应稳定返回 conflict，而不是每轮重新算一遍。
func TestSyncPushConflictKeepsIdempotencyRecord(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()
	payload, err := json.Marshal(SyncPayload{Title: "冲突分组"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// 先建一次，拿到当前修订号
	first, err := f.s.SyncPush(ctx, f.admin.ID, model.UserTypeSuperuser, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev",
		Ops: []SyncOpIn{{OpID: "op-c1", Op: model.SyncOpUpsert, Type: model.SyncTypeGroup, UID: "uid-conf", Payload: payload}},
	})
	if err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	cur := first.Results[0].ItemRev

	// 用过期的 base_rev 再推一次 ⇒ conflict
	resp, err := f.s.SyncPush(ctx, f.admin.ID, model.UserTypeSuperuser, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev",
		Ops: []SyncOpIn{{
			OpID: "op-c2", Op: model.SyncOpUpsert, Type: model.SyncTypeGroup,
			UID: "uid-conf", BaseRev: cur - 1, Payload: payload,
		}},
	})
	if err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	if resp.Results[0].Status != "conflict" {
		t.Fatalf("过期 base_rev 应返回 conflict，got %+v", resp.Results[0])
	}
	if resp.Results[0].ItemRev != cur {
		t.Fatalf("conflict 应回传服务端当前修订号 %d，got %d", cur, resp.Results[0].ItemRev)
	}
	if !hasSyncOp(t, f, "op-c2") {
		t.Fatal("conflict 也要落幂等记录，避免每轮重算")
	}
}

// TestSyncPushRollsBackEntityWhenIdempotencyWriteFails
// 把 sync_op 表删掉，让幂等记录写入必然失败：此时实体写入必须一起回滚，
// 否则客户端收到 rejected 去重试时，第一轮的实体已经躺在库里了（重复创建）。
func TestSyncPushRollsBackEntityWhenIdempotencyWriteFails(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()
	if err := f.s.repo.DB().Exec("DROP TABLE sync_op").Error; err != nil {
		t.Fatalf("drop sync_op: %v", err)
	}
	payload, err := json.Marshal(SyncPayload{Title: "不该存在"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := f.s.SyncPush(ctx, f.admin.ID, model.UserTypeSuperuser, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev",
		Ops: []SyncOpIn{{OpID: "op-drop", Op: model.SyncOpUpsert, Type: model.SyncTypeGroup, UID: "uid-drop", Payload: payload}},
	})
	if err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	if resp.Results[0].Status != "rejected" {
		t.Fatalf("幂等记录写失败应 rejected，got %+v", resp.Results[0])
	}
	g, err := f.s.repo.FindGroupByExtUID(ctx, f.proj.ID, "uid-drop")
	if err == nil && g != nil {
		t.Fatalf("幂等记录失败时实体必须回滚，但分组已存在: %+v", g)
	}
}

// TestSyncEndpointsFailLoudWhenCursorBroken P2-5：游标读不出来时必须报错。
// 静默回落 0 会让客户端以为服务端是空的，下一轮从 0 全量重拉。
func TestSyncEndpointsFailLoudWhenCursorBroken(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()
	// 造一条流水，保证正常路径下能读出非 0 游标
	payload, err := json.Marshal(SyncPayload{Title: "游标分组"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := f.s.SyncPush(ctx, f.admin.ID, model.UserTypeSuperuser, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev",
		Ops: []SyncOpIn{{OpID: "op-cur", Op: model.SyncOpUpsert, Type: model.SyncTypeGroup, UID: "uid-cur", Payload: payload}},
	}); err != nil {
		t.Fatalf("SyncPush: %v", err)
	}
	// 破坏流水表：此后 LatestChangeID 必然失败
	if err := f.s.repo.DB().Exec("DROP TABLE change_log").Error; err != nil {
		t.Fatalf("drop change_log: %v", err)
	}
	if _, err := f.s.SyncSnapshot(ctx, f.admin.ID, model.UserTypeSuperuser, f.proj.ID, 0, 10); err == nil {
		t.Fatal("快照取不到游标时应报错，而不是返回 cursor=0")
	}
	if _, err := f.s.SyncChanges(ctx, f.admin.ID, model.UserTypeSuperuser, f.proj.ID, 0, 10); err == nil {
		t.Fatal("增量取不到游标时应报错")
	}
	if _, err := f.s.SyncPush(ctx, f.admin.ID, model.UserTypeSuperuser, &SyncPushReq{
		ProjectID: f.proj.ID, DeviceID: "dev",
		Ops: []SyncOpIn{{OpID: "op-cur2", Op: model.SyncOpUpsert, Type: model.SyncTypeGroup, UID: "uid-cur2", Payload: payload}},
	}); err == nil {
		t.Fatal("写成功后读不到游标时应让本次响应失败，避免客户端误判从小游标重拉")
	}
}
