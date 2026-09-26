package service

import (
	"context"
	"errors"
	"testing"

	"apidoc/internal/dto"
	"apidoc/internal/model"
	"apidoc/internal/pkg/errcode"
)

// TestRestoreBatchWithGroups 覆盖回收站批量恢复：
// 恢复接口时会把「已删除的所属分组（含祖先链）」一并恢复；权限不足则拒绝。
func TestRestoreBatchWithGroups(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()

	parent, err := f.s.CreateGroup(ctx, f.admin.ID, f.admin.Type, dto.GroupCreateReq{
		ProjectID: f.proj.ID, Title: "parent", Type: model.GroupTypeAPI,
	})
	if err != nil {
		t.Fatalf("CreateGroup parent: %v", err)
	}
	child, err := f.s.CreateGroup(ctx, f.admin.ID, f.admin.Type, dto.GroupCreateReq{
		ProjectID: f.proj.ID, PID: parent.ID, Title: "child", Type: model.GroupTypeAPI,
	})
	if err != nil {
		t.Fatalf("CreateGroup child: %v", err)
	}
	api, err := f.s.CreateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, GroupID: child.ID, ApiName: "restore-api", HttpMethodType: "GET", URL: "https://api/x",
	})
	if err != nil {
		t.Fatalf("CreateApi: %v", err)
	}

	// 删除接口 → 删除子分组 → 删除父分组（模拟一次「清空项目」）。
	if err := f.s.DeleteApi(ctx, f.admin.ID, f.admin.Type, api.ID); err != nil {
		t.Fatalf("DeleteApi: %v", err)
	}
	if err := f.s.DeleteGroup(ctx, f.admin.ID, f.admin.Type, child.ID); err != nil {
		t.Fatalf("DeleteGroup child: %v", err)
	}
	if err := f.s.DeleteGroup(ctx, f.admin.ID, f.admin.Type, parent.ID); err != nil {
		t.Fatalf("DeleteGroup parent: %v", err)
	}

	active, err := f.s.ListApis(ctx, dto.ApiListQuery{ProjectID: f.proj.ID, PageNum: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListApis: %v", err)
	}
	if active.Total != 0 {
		t.Fatalf("删除后不应有活跃接口，got %d", active.Total)
	}

	// 只读成员无权恢复。
	bob := f.registerReadonly(t, "bob")
	if _, err := f.s.RestoreApis(ctx, bob.ID, bob.Type, []uint64{api.ID}); err == nil {
		t.Fatal("只读成员恢复接口应被拒绝")
	} else {
		var be *BizError
		if !errors.As(err, &be) || be.Code != errcode.CodeForbidden {
			t.Fatalf("应为无权限错误，got %v", err)
		}
	}

	// 管理员恢复：分组链一并回来。
	res, err := f.s.RestoreApis(ctx, f.admin.ID, f.admin.Type, []uint64{api.ID})
	if err != nil {
		t.Fatalf("RestoreApis: %v", err)
	}
	if res.Restored != 1 || res.Groups != 2 {
		t.Fatalf("应恢复 1 个接口与 2 个分组，got %+v", res)
	}
	groups, err := f.s.repo.GroupsByProjectsAny(ctx, []uint64{f.proj.ID}, model.GroupTypeAPI)
	if err != nil {
		t.Fatalf("GroupsByProjectsAny: %v", err)
	}
	for _, g := range groups {
		if g.ID == parent.ID || g.ID == child.ID {
			if g.IsDeleted != 0 {
				t.Fatalf("分组 %d 应已恢复", g.ID)
			}
		}
	}
	active, err = f.s.ListApis(ctx, dto.ApiListQuery{ProjectID: f.proj.ID, PageNum: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListApis: %v", err)
	}
	if active.Total != 1 || active.Items[0].GroupID != child.ID {
		t.Fatalf("恢复后接口应回到原子分组，got %+v", active.Items)
	}

	// 批量：一次恢复多条（此处再删两条后一起恢复）。
	api2, err := f.s.CreateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiCreateReq{
		ProjectID: f.proj.ID, GroupID: child.ID, ApiName: "restore-api-2", HttpMethodType: "POST", URL: "https://api/y",
	})
	if err != nil {
		t.Fatalf("CreateApi 2: %v", err)
	}
	for _, id := range []uint64{api.ID, api2.ID} {
		if err := f.s.DeleteApi(ctx, f.admin.ID, f.admin.Type, id); err != nil {
			t.Fatalf("DeleteApi: %v", err)
		}
	}
	if res, err = f.s.RestoreApis(ctx, f.admin.ID, f.admin.Type, []uint64{api.ID, api2.ID}); err != nil {
		t.Fatalf("RestoreApis batch: %v", err)
	} else if res.Restored != 2 {
		t.Fatalf("批量恢复应为 2 条，got %+v", res)
	}

	// 空列表与不存在 id：返回参数错误。
	if _, err := f.s.RestoreApis(ctx, f.admin.ID, f.admin.Type, nil); err == nil {
		t.Fatal("空 ids 应报参数错误")
	}
	if _, err := f.s.RestoreApis(ctx, f.admin.ID, f.admin.Type, []uint64{999999}); err == nil {
		t.Fatal("不存在的 id 应报参数错误")
	}
}

// TestPurgeBatch 覆盖回收站彻底删除：行从库中消失（不是软删除）。
func TestPurgeBatch(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()
	api := f.newAPI(t, "purge-api", "https://api/purge", "")
	if err := f.s.DeleteApi(ctx, f.admin.ID, f.admin.Type, api.ID); err != nil {
		t.Fatalf("DeleteApi: %v", err)
	}
	n, err := f.s.PurgeApis(ctx, f.admin.ID, f.admin.Type, []uint64{api.ID})
	if err != nil {
		t.Fatalf("PurgeApis: %v", err)
	}
	if n != 1 {
		t.Fatalf("应彻底删除 1 条，got %d", n)
	}
	var count int64
	if err := f.s.repo.DB().Raw("SELECT COUNT(*) FROM api WHERE id = ?", api.ID).Scan(&count).Error; err != nil {
		t.Fatalf("查询 api 表: %v", err)
	}
	if count != 0 {
		t.Fatalf("彻底删除后库中不应再有该行，got %d", count)
	}
	// 只读成员无权彻底删除。
	doc, err := f.s.CreateDoc(ctx, f.admin.ID, f.admin.Type, dto.DocCreateReq{ProjectID: f.proj.ID, Title: "purge-doc", Content: "x"})
	if err != nil {
		t.Fatalf("CreateDoc: %v", err)
	}
	if err := f.s.DeleteDoc(ctx, f.admin.ID, f.admin.Type, doc.ID); err != nil {
		t.Fatalf("DeleteDoc: %v", err)
	}
	bob := f.registerReadonly(t, "bob")
	if _, err := f.s.PurgeDocs(ctx, bob.ID, bob.Type, []uint64{doc.ID}); err == nil {
		t.Fatal("只读成员彻底删除文档应被拒绝")
	}
}
