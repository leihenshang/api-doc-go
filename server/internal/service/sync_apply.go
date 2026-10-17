package service

// upsert / delete 的落库分支（P2-14：从 sync.go 拆出）。
// 六种实体的字段映射都在本文件；协议结构见 share/syncapi。

import (
	"context"
	"encoding/json"

	"apidoc/internal/model"
	"apidoc/internal/repository"
)

func (s *Service) applyUpsert(ctx context.Context, r *repository.Repository, userID uint64, req *SyncPushReq, op SyncOpIn) SyncOpOut {
	var p SyncPayload
	if len(op.Payload) > 0 {
		// P0-3：解析失败必须拒收（rejected），不能用零值覆盖存量数据——
		// 否则一次畸形 payload 就会把服务端上的标题/内容/请求参数静默清空。
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "bad payload"}
		}
	}
	device := req.DeviceID
	if req.DeviceName != "" {
		device = req.DeviceName
	}

	switch op.Type {
	case model.SyncTypeGroup:
		e, lerr := r.FindGroupByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			e = &model.Group{ProjectID: req.ProjectID, Type: model.GroupTypeAPI}
		}
		e.ExtUID = op.UID
		e.Title = p.Title
		e.Priority = p.Priority
		if p.PID != "" {
			if pg, _ := r.FindGroupByExtUID(ctx, req.ProjectID, p.PID); pg != nil {
				e.PID = pg.ID
			} else {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
		} else {
			e.PID = 0
		}
		var err error
		if e.ID == 0 {
			err = r.CreateGroup(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateGroup(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeGroup, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}

	case model.SyncTypeAPI:
		e, lerr := r.FindApiByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			e = &model.Api{ProjectID: req.ProjectID}
		}
		e.ExtUID = op.UID
		e.ApiName = p.Name
		e.HttpMethodType = p.Method
		e.URL = p.URL
		if p.GroupUID != "" {
			if g, _ := r.FindGroupByExtUID(ctx, req.ProjectID, p.GroupUID); g != nil {
				e.GroupID = g.ID
			} else {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
		} else {
			e.GroupID = 0
		}
		applyApiExtra(e, p.Extra)
		var err error
		if e.ID == 0 {
			err = r.CreateApi(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateApi(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeAPI, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}

	case model.SyncTypeDoc:
		e, lerr := r.FindDocByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			e = &model.Doc{ProjectID: req.ProjectID, UserID: userID}
		}
		e.ExtUID = op.UID
		e.Title = p.Title
		e.Content = p.Content
		e.Icon = p.Icon
		if p.GroupUID != "" {
			if g, _ := r.FindGroupByExtUID(ctx, req.ProjectID, p.GroupUID); g != nil {
				e.GroupID = g.ID
			} else {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
		} else {
			e.GroupID = 0
		}
		var err error
		if e.ID == 0 {
			err = r.CreateDoc(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateDoc(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeDoc, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}

	case model.SyncTypeExample:
		e, lerr := r.FindApiReturnSampleByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			e = &model.ApiReturnSample{ProjectID: req.ProjectID}
		}
		e.ExtUID = op.UID
		e.Name = p.Name
		e.SampleText = p.Sample
		e.Status = p.Status
		e.ContentType = p.ContentType
		e.Seq = p.Seq
		if p.APIUID != "" {
			if a, _ := r.FindApiByExtUID(ctx, req.ProjectID, p.APIUID); a != nil {
				e.ApiID = a.ID
			} else {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
		}
		var err error
		if e.ID == 0 {
			err = r.CreateApiReturnSample(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateApiReturnSample(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeExample, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}

	case model.SyncTypeVarEnv:
		// 环境：Name 复用载荷字段；仅建/改同步身份与名称，颜色/描述等 Web 侧属性不覆盖。
		e, lerr := r.FindVarEnvByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			e = &model.VariableEnv{ProjectID: req.ProjectID}
		}
		e.ExtUID = op.UID
		if p.Name != "" {
			e.Name = p.Name
		}
		var err error
		if e.ID == 0 {
			err = r.CreateVariableEnv(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateVariableEnv(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeVarEnv, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}

	case model.SyncTypeVariable:
		// 变量：项目级、挂到 env_uid 对应环境；Value 原样存（secret 为 enc:v1: 密文，服务端不解析）。
		e, lerr := r.FindVariableByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			e = &model.Variable{ProjectID: req.ProjectID, Scope: model.VarScopeProject}
		}
		e.ExtUID = op.UID
		e.Name = p.Name
		e.Value = p.Value
		e.IsSecret = model.VarNotSecret
		if p.Secret {
			e.IsSecret = model.VarSecret
		}
		e.Enabled = model.VarEnabled
		if p.Enabled != nil && !*p.Enabled {
			e.Enabled = model.VarDisabled
		}
		e.EnvID = 0
		if p.EnvUID != "" {
			env, _ := r.FindVarEnvByExtUID(ctx, req.ProjectID, p.EnvUID)
			if env == nil {
				return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "parent_missing"}
			}
			e.EnvID = env.ID
		}
		var err error
		if e.ID == 0 {
			err = r.CreateVariable(ctx, e)
		} else {
			e.IsDeleted = model.IsDeletedNo
			err = r.UpdateVariable(ctx, e)
		}
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
		cid, err := s.appendChangeTx(ctx, r, model.SyncTypeVariable, op.UID, req.ProjectID, e.ID, model.SyncOpUpsert, userID, device)
		if err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
		}
		e.LastChangeID = cid
		return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}
	}
	return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
}

func (s *Service) applyDelete(ctx context.Context, r *repository.Repository, userID uint64, req *SyncPushReq, op SyncOpIn) SyncOpOut {
	device := req.DeviceID
	if req.DeviceName != "" {
		device = req.DeviceName
	}
	var id uint64
	switch op.Type {
	case model.SyncTypeGroup:
		e, lerr := r.FindGroupByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteGroup(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeAPI:
		e, lerr := r.FindApiByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteApi(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeDoc:
		e, lerr := r.FindDocByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteDoc(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeExample:
		e, lerr := r.FindApiReturnSampleByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteApiReturnSample(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeVarEnv:
		e, lerr := r.FindVarEnvByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteVariableEnv(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	case model.SyncTypeVariable:
		e, lerr := r.FindVariableByExtUID(ctx, req.ProjectID, op.UID)
		// P1-3：查询出错（非「不存在」）必须中止，否则会被当成新建导致重复/覆盖
		if out, bad := lookupRejected(op, lerr); bad {
			return out
		}
		if e == nil {
			return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: 0}
		}
		id = e.ID
		if err := r.DeleteVariable(ctx, id); err != nil {
			return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
		}
	default:
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "invalid"}
	}
	cid, err := s.appendChangeTx(ctx, r, op.Type, op.UID, req.ProjectID, id, model.SyncOpDelete, userID, device)
	if err != nil {
		return SyncOpOut{OpID: op.OpID, Status: "rejected", Reason: "变更流水写入失败"}
	}
	return SyncOpOut{OpID: op.OpID, Status: "ok", ItemRev: cid}
}
