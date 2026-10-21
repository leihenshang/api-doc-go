package service

// 2026-10-19 服务端审核 High 组的回归测试（H-1〜H-8）。

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/repository"
)

// H-1：禁用账号的存量登录态在回源刷新时被拒；正常账号返回当前类型。
func TestLoadPrincipalRefreshesState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestService(t)
	admin, err := s.InitAdmin(ctx, "admin", "admin123")
	if err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	bob, err := s.Register(ctx, dto.RegisterReq{Name: "bob", Pwd: "p123456", RePwd: "p123456"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	typ, err := s.LoadPrincipal(ctx, bob.ID)
	if err != nil || typ != model.UserTypeNormal {
		t.Fatalf("正常账号应通过并返回普通用户：typ=%d err=%v", typ, err)
	}
	// 管理员禁用 bob 后，bob 已签发的 JWT 不应再被放行。
	if _, err := s.UpdateUser(ctx, *admin, bob.ID, dto.UserUpdateReq{State: model.UserStateDisabled}); err != nil {
		t.Fatalf("禁用 bob: %v", err)
	}
	if _, err := s.LoadPrincipal(ctx, bob.ID); err == nil {
		t.Fatal("禁用账号的存量登录态应被拒绝")
	}
}

// H-2/H-3：同步分页的 limit 必须在入口收敛，且与 repository 的上界同口径。
func TestSyncLimitClamps(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want int }{
		{0, 200}, {-5, 200}, {50, 50}, {1000, 1000}, {5000, 1000},
	} {
		if got := clampChangeLimit(c.in); got != c.want {
			t.Errorf("clampChangeLimit(%d)=%d want %d", c.in, got, c.want)
		}
	}
	for _, c := range []struct{ in, want int }{
		{0, 200}, {-1, 200}, {100, 100}, {500, 500}, {1000000, 200},
	} {
		if got := clampSnapshotLimit(c.in); got != c.want {
			t.Errorf("clampSnapshotLimit(%d)=%d want %d", c.in, got, c.want)
		}
	}
}

// H-4：更新分组父级时成环必须被拒（此前可直接制造 p_id 环）。
func TestUpdateGroupRejectsCycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestService(t)
	admin, _ := s.InitAdmin(ctx, "admin", "admin123")
	proj, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "cycle"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	a, err := s.CreateGroup(ctx, admin.ID, admin.Type, dto.GroupCreateReq{ProjectID: proj.ID, Title: "A"})
	if err != nil {
		t.Fatalf("CreateGroup A: %v", err)
	}
	b, err := s.CreateGroup(ctx, admin.ID, admin.Type, dto.GroupCreateReq{ProjectID: proj.ID, PID: a.ID, Title: "B"})
	if err != nil {
		t.Fatalf("CreateGroup B: %v", err)
	}
	// A → B（B 是 A 的子级）应成环被拒。
	if _, err := s.UpdateGroup(ctx, admin.ID, admin.Type, dto.GroupUpdateReq{ID: a.ID, PID: b.ID}); err == nil {
		t.Fatal("把父分组挂到自己的子分组下应被拒绝")
	}
	// 自指同样拒绝。
	if _, err := s.UpdateGroup(ctx, admin.ID, admin.Type, dto.GroupUpdateReq{ID: a.ID, PID: a.ID}); err == nil {
		t.Fatal("把分组挂到自身应被拒绝")
	}
}

// H-5：Bruno 导入的目录层级超过上限必须报错（此前无界递归会栈溢出崩进程）。
func TestBrunoImportDepthLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestService(t)
	admin, _ := s.InitAdmin(ctx, "admin", "admin123")
	proj, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "deep"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	deep := strings.Repeat("d/", maxImportDepth+5) + "req.yml"
	files := []dto.BrunoFile{{Path: deep, Content: "info:\n  name: r\n  type: http\nhttp:\n  method: GET\n  url: http://x/a\n"}}
	if _, err := s.ImportBruno(ctx, admin.ID, admin.Type, proj.ID, "append", files); err == nil {
		t.Fatal("超深目录导入应报错而不是继续递归")
	}
}

// H-7：转让负责人必须清掉旧负责人（否则每次转让都累积一个全权限账号）。
func TestSetLeaderClearsPrevious(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestService(t)
	admin, _ := s.InitAdmin(ctx, "admin", "admin123")
	bob, _ := s.Register(ctx, dto.RegisterReq{Name: "bob", Pwd: "p123456", RePwd: "p123456"})
	carol, _ := s.Register(ctx, dto.RegisterReq{Name: "carol", Pwd: "p123456", RePwd: "p123456"})
	proj, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "leader"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	for _, uid := range []uint64{bob.ID, carol.ID} {
		if err := s.AddMember(ctx, admin.ID, admin.Type, dto.MemberAddReq{ProjectID: proj.ID, UserID: uid}); err != nil {
			t.Fatalf("AddMember %d: %v", uid, err)
		}
	}
	if err := s.SetLeader(ctx, admin.ID, admin.Type, dto.SetLeaderReq{ProjectID: proj.ID, UserID: bob.ID}); err != nil {
		t.Fatalf("SetLeader bob: %v", err)
	}
	if err := s.SetLeader(ctx, admin.ID, admin.Type, dto.SetLeaderReq{ProjectID: proj.ID, UserID: carol.ID}); err != nil {
		t.Fatalf("SetLeader carol: %v", err)
	}
	views, err := s.ListMembers(ctx, admin.ID, admin.Type, proj.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	leaders := 0
	for _, v := range views {
		if v.IsLeader == 1 {
			leaders++
			if v.UserID != carol.ID {
				t.Fatalf("旧负责人未清理：%d 仍是 leader", v.UserID)
			}
		}
	}
	if leaders != 1 {
		t.Fatalf("应恰好一个负责人，实际 %d", leaders)
	}
}

// H-9：编辑变量不得重新生成 ext_uid（同步身份必须稳定）。
func TestUpdateVariableKeepsExtUID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestService(t)
	admin, _ := s.InitAdmin(ctx, "admin", "admin123")
	proj, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "var"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	val := "v1"
	v, err := s.CreateVariable(ctx, admin.ID, admin.Type, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: proj.ID, Name: "token", Value: &val,
	})
	if err != nil {
		t.Fatalf("CreateVariable: %v", err)
	}
	before, err := s.repo.GetVariable(ctx, v.ID)
	if err != nil || before.ExtUID == "" {
		t.Fatalf("新建变量应有 ext_uid: %v %+v", err, before)
	}
	val2 := "v2"
	if _, err := s.UpdateVariable(ctx, admin.ID, admin.Type, dto.VariableReq{
		ID: v.ID, Scope: model.VarScopeProject, ProjectID: proj.ID, Name: "token", Value: &val2,
	}); err != nil {
		t.Fatalf("UpdateVariable: %v", err)
	}
	after, err := s.repo.GetVariable(ctx, v.ID)
	if err != nil {
		t.Fatalf("重读变量: %v", err)
	}
	if after.ExtUID != before.ExtUID {
		t.Fatalf("更新后 ext_uid 应不变：%q → %q", before.ExtUID, after.ExtUID)
	}
}

// H-10：变量/环境/响应示例改软删——业务读不到、快照带 tombstone、到期可物理清理。
func TestSoftDeleteAndPurge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestService(t)
	admin, _ := s.InitAdmin(ctx, "admin", "admin123")
	proj, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "soft"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// 变量：删除后业务读不到，但快照仍有 Deleted 墓碑。
	val := "v"
	v, err := s.CreateVariable(ctx, admin.ID, admin.Type, dto.VariableReq{
		Scope: model.VarScopeProject, ProjectID: proj.ID, Name: "k", Value: &val,
	})
	if err != nil {
		t.Fatalf("CreateVariable: %v", err)
	}
	created, err := s.repo.GetVariable(ctx, v.ID)
	if err != nil {
		t.Fatalf("重读变量: %v", err)
	}
	uid := created.ExtUID
	if err := s.DeleteVariable(ctx, admin.ID, admin.Type, v.ID); err != nil {
		t.Fatalf("DeleteVariable: %v", err)
	}
	if _, err := s.repo.GetVariable(ctx, v.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("删除后业务读路径应视为不存在，got %v", err)
	}
	snap, err := s.SyncSnapshot(ctx, admin.ID, admin.Type, proj.ID, 0, 200)
	if err != nil {
		t.Fatalf("SyncSnapshot: %v", err)
	}
	seen := false
	for _, it := range snap.Items {
		if it.Type == model.SyncTypeVariable && it.UID == uid {
			seen = true
			if !it.Deleted {
				t.Fatal("软删变量应在快照中标记 Deleted=true")
			}
		}
	}
	if !seen {
		t.Fatal("快照应包含软删变量的墓碑")
	}

	// 环境：同样软删。
	env, err := s.CreateVariableEnv(ctx, admin.ID, admin.Type, dto.VariableEnvReq{ProjectID: proj.ID, Name: "dev"})
	if err != nil {
		t.Fatalf("CreateVariableEnv: %v", err)
	}
	if err := s.DeleteVariableEnv(ctx, admin.ID, admin.Type, env.ID); err != nil {
		t.Fatalf("DeleteVariableEnv: %v", err)
	}
	if _, err := s.repo.GetVariableEnv(ctx, env.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("删除后环境应不可读，got %v", err)
	}

	// 响应示例：仓储层软删 + 物理清理。
	ex := &model.ApiReturnSample{ProjectID: proj.ID, ApiID: 1, ExtUID: "ex-uid", Seq: 1}
	if err := s.repo.CreateApiReturnSample(ctx, ex); err != nil {
		t.Fatalf("CreateApiReturnSample: %v", err)
	}
	if err := s.repo.DeleteApiReturnSample(ctx, ex.ID); err != nil {
		t.Fatalf("DeleteApiReturnSample: %v", err)
	}
	if _, err := s.repo.GetApiReturnSample(ctx, ex.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("软删示例业务读应不存在，got %v", err)
	}
	all, err := s.repo.ListApiReturnSamplesAll(ctx, proj.ID)
	if err != nil || len(all) != 1 || all[0].IsDeleted != model.IsDeletedYes {
		t.Fatalf("同步用查询应含软删示例：%v %+v", err, all)
	}

	// 到期物理清理（截止时间取未来，模拟已过保留期）。
	if _, err := s.repo.PurgeDeletedVariables(ctx, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("PurgeDeletedVariables: %v", err)
	}
	if _, err := s.repo.PurgeDeletedVariableEnvs(ctx, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("PurgeDeletedVariableEnvs: %v", err)
	}
	if _, err := s.repo.PurgeDeletedApiReturnSamples(ctx, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("PurgeDeletedApiReturnSamples: %v", err)
	}
	snap2, err := s.SyncSnapshot(ctx, admin.ID, admin.Type, proj.ID, 0, 200)
	if err != nil {
		t.Fatalf("SyncSnapshot after purge: %v", err)
	}
	for _, it := range snap2.Items {
		if it.Type == model.SyncTypeVariable && it.UID == uid {
			t.Fatal("物理清理后墓碑应从快照消失")
		}
	}
	if left, _ := s.repo.ListApiReturnSamplesAll(ctx, proj.ID); len(left) != 0 {
		t.Fatalf("物理清理后示例应移除：%+v", left)
	}
}

// H-8：Postman 导入产出的接口必须带 ext_uid（否则同步永远拉不到）。
func TestPostmanImportSetsExtUID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestService(t)
	admin, _ := s.InitAdmin(ctx, "admin", "admin123")
	proj, err := s.CreateProject(ctx, admin.ID, dto.ProjectCreateReq{Title: "pm"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	body := []byte(`{"info":{"name":"c","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},` +
		`"item":[{"name":"r1","request":{"method":"GET","url":"http://x.example/a"}}]}`)
	if _, err := s.ImportPostman(ctx, admin.ID, admin.Type, proj.ID, "append", body); err != nil {
		t.Fatalf("ImportPostman: %v", err)
	}
	apis, err := s.repo.ListApisByProject(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListApisByProject: %v", err)
	}
	if len(apis) == 0 {
		t.Fatal("导入后应至少有一条接口")
	}
	for _, a := range apis {
		if strings.TrimSpace(a.ExtUID) == "" {
			t.Fatalf("导入接口缺少 ext_uid：%+v", a)
		}
	}
}
