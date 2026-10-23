package service

// push 入口与逐条应用（P2-14：从 sync.go 拆出）。
// 事务边界见 execOp：实体写入 + 变更流水 + 修订号 + 幂等记录必须同生共死。

import (
	"context"
	"errors"

	"apidoc/internal/model"
	"apidoc/internal/repository"
)

// hasDeleteOp 判断批次中是否含删除操作（用于追加 delete 权限位校验）。
func hasDeleteOp(ops []SyncOpIn) bool {
	for _, op := range ops {
		if op.Op == model.SyncOpDelete {
			return true
		}
	}
	return false
}

// SyncPush 批量提交（幂等 + 拓扑序 + 冲突检测）。
func (s *Service) SyncPush(ctx context.Context, userID uint64, typ int8, req *SyncPushReq) (*SyncPushResp, error) {
	if req == nil || req.ProjectID == 0 {
		return nil, errors.New("project_id 必填")
	}
	// 越权写防护：此前无任何权限校验，任意登录用户/PAT 可改写甚至物理删除任意项目数据。
	// 写操作需 update 位；批次中含删除操作时再校验 delete 位。
	if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermUpdate); err != nil {
		return nil, err
	}
	if hasDeleteOp(req.Ops) {
		if err := s.ensureProjectPermission(ctx, userID, typ, req.ProjectID, model.PermDelete); err != nil {
			return nil, err
		}
	}
	// 写开关（S14）
	proj, err := s.repo.GetProjectAny(ctx, req.ProjectID)
	if err != nil {
		return nil, err
	}
	// 注：权限位已在函数开头校验过（update 位，含删除时的 delete 位），此处不再重复调用——
	// 同一请求里连做两次相同的权限查询只是多一次 DB 往返（P2-3）。
	writeDisabled := proj != nil && proj.ClientWriteEnabled != nil && !*proj.ClientWriteEnabled

	out := make([]SyncOpOut, 0, len(req.Ops))
	for _, op := range req.Ops {
		// 幂等
		if prev, err := s.repo.GetSyncOp(ctx, op.OpID); err == nil && prev != nil {
			out = append(out, SyncOpOut{
				OpID: op.OpID, Status: prev.Status, ItemRev: prev.ItemRev, Reason: prev.Reason,
			})
			continue
		}
		out = append(out, s.execOp(ctx, userID, req, op, writeDisabled))
	}
	// P2-5：批量写入已经提交，但游标读不出来时不能假装 cursor=0 ——
	// 客户端会以为自己的游标最旧，下一轮从 0 全量重拉。宁可让本次响应失败，让它重试。
	cur, err := s.repo.LatestChangeID(ctx, req.ProjectID)
	if err != nil {
		return nil, err
	}
	return &SyncPushResp{Results: out, Cursor: cur}, nil
}

// errOpRollback 只用于在事务回调里请求回滚：业务结果已经写进 res，不需要真的返回错误。
var errOpRollback = errors.New("sync: op rejected, rollback")

// execOp 在**单个事务**内应用一条 op：实体写入 + 变更流水 + 修订号 + 幂等记录。
//
// P1-1：此前这三步各自独立提交。第 2 步失败会留下「数据已改但没有流水」的条目
// （客户端永远拉不到），第 3 步失败会留下「没有幂等记录」的条目（重放时被再执行一遍）。
// 现在任一步失败整条 op 回滚，客户端拿到 rejected 后重试是安全的。
//
// 业务性拒绝（conflict / rejected / readonly）同样回滚：它们已经写了一半的实体必须撤销，
// 只有 ok 与 conflict 需要留下幂等记录（conflict 的重复推送应稳定返回 conflict，而不是重算）。
func (s *Service) execOp(ctx context.Context, userID uint64, req *SyncPushReq, op SyncOpIn, writeDisabled bool) SyncOpOut {
	var res SyncOpOut
	err := s.repo.Transaction(ctx, func(tx *repository.Repository) error {
		res = s.applyOp(ctx, tx, userID, req, op, writeDisabled)
		if res.Status != "ok" && res.Status != "conflict" {
			return errOpRollback
		}
		if err := tx.SaveSyncOp(ctx, &model.SyncOp{
			OpID: op.OpID, ProjectID: req.ProjectID,
			Status: res.Status, ItemRev: res.ItemRev, Reason: res.Reason,
		}); err != nil {
			res = SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "idempotency record failed"}
			return errOpRollback
		}
		return nil
	})
	if err != nil && !errors.Is(err, errOpRollback) {
		// 事务本身失败（提交出错、锁超时等）：不能谎报成功，让客户端重试。
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "transaction failed"}
	}
	return res
}

// applyOp 处理单条 push。
func (s *Service) applyOp(ctx context.Context, r *repository.Repository, userID uint64, req *SyncPushReq, op SyncOpIn, writeDisabled bool) SyncOpOut {
	if writeDisabled {
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "readonly"}
	}
	switch op.Type {
	case model.SyncTypeGroup, model.SyncTypeAPI, model.SyncTypeDoc, model.SyncTypeExample,
		model.SyncTypeVarEnv, model.SyncTypeVariable, model.SyncTypeFieldMapping:
	default:
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
	}
	if op.UID == "" {
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
	}

	// 冲突检测：base_rev != last_change_id ⇒ conflict（新建 base_rev=0 允许）
	var curRev int64
	var exists bool
	switch op.Type {
	case model.SyncTypeGroup:
		e, lerr := r.FindGroupByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeAPI:
		e, lerr := r.FindApiByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeDoc:
		e, lerr := r.FindDocByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeExample:
		e, lerr := r.FindApiReturnSampleByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeVarEnv:
		e, lerr := r.FindVarEnvByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeVariable:
		e, lerr := r.FindVariableByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	case model.SyncTypeFieldMapping:
		e, lerr := r.FindFieldMappingByExtUID(ctx, req.ProjectID, op.UID)
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e != nil {
			exists, curRev = true, e.LastChangeID
		}
	}
	if exists && op.BaseRev > 0 && op.BaseRev != curRev {
		// push 入口已校验 PermUpdate，等价于具备明文权限：冲突回传服务端原文。
		payload, _ := s.loadPayload(ctx, req.ProjectID, op.Type, op.UID, true)
		return SyncOpOut{
			OpID: op.OpID, Status: "conflict", ItemRev: curRev, Server: payload,
		}
	}

	if op.Op == model.SyncOpDelete {
		return s.applyDelete(ctx, r, userID, req, op)
	}
	return s.applyUpsert(ctx, r, userID, req, op)
}

// lookupRejected 查询实体出错时返回 rejected（P1-3）：
// 只有确实不存在才允许走新建分支；DB 错误被当成不存在会导致重复创建或整条覆盖。
func lookupRejected(op SyncOpIn, err error) (SyncOpOut, bool) {
	if err == nil || errors.Is(err, repository.ErrNotFound) {
		return SyncOpOut{}, false
	}
	return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "lookup failed"}, true
}
