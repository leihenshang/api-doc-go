package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"apidoc/internal/dto"
	"apidoc/internal/pkg/errcode"
)

// TestOptimisticLock 覆盖并发编辑保护（接口与文档）：
// 传入载入时的 update_time 时，若他人已保存则拒绝；传当前值可保存；不传该字段保持历史行为。
func TestOptimisticLock(t *testing.T) {
	f := newVarFixture(t)
	ctx := context.Background()

	api := f.newAPI(t, "lock-api", "https://api/x", "")
	// 模拟前端：保存前先 GET 一次，用返回的 update_time 作为期望值。
	loaded, err := f.s.GetApi(ctx, f.admin.ID, f.admin.Type, api.ID)
	if err != nil {
		t.Fatalf("GetApi: %v", err)
	}
	stale := loaded.UpdateTime.Format(time.RFC3339Nano)

	// 1) 带当前 update_time 保存成功。
	if _, err := f.s.UpdateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiUpdateReq{
		ID:                 api.ID,
		ExpectedUpdateTime: stale,
		ApiCreateReq: dto.ApiCreateReq{
			ProjectID: f.proj.ID, ApiName: "lock-api-2", HttpMethodType: "POST", URL: "https://api/y",
		},
	}); err != nil {
		t.Fatalf("首次保存应成功: %v", err)
	}

	// 2) 用旧 update_time 再保存 → 冲突（他人已修改）。
	_, err = f.s.UpdateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiUpdateReq{
		ID:                 api.ID,
		ExpectedUpdateTime: stale,
		ApiCreateReq: dto.ApiCreateReq{
			ProjectID: f.proj.ID, ApiName: "lock-api-3", HttpMethodType: "POST", URL: "https://api/z",
		},
	})
	var be *BizError
	if !errors.As(err, &be) || be.Code != errcode.CodeFailed {
		t.Fatalf("旧版本保存应报冲突，got %v", err)
	}
	if !strings.Contains(be.Msg, "已被他人修改") {
		t.Fatalf("冲突文案应提示已被他人修改，got %q", be.Msg)
	}
	// 冲突时不应写入。
	after, err := f.s.GetApi(ctx, f.admin.ID, f.admin.Type, api.ID)
	if err != nil {
		t.Fatalf("GetApi: %v", err)
	}
	if after.ApiName != "lock-api-2" {
		t.Fatalf("冲突时不应覆盖，got %q", after.ApiName)
	}

	// 3) 不传 expected_update_time → 保持历史行为（可保存）。
	if _, err := f.s.UpdateApi(ctx, f.admin.ID, f.admin.Type, dto.ApiUpdateReq{
		ID: api.ID,
		ApiCreateReq: dto.ApiCreateReq{
			ProjectID: f.proj.ID, ApiName: "lock-api-4", HttpMethodType: "POST", URL: "https://api/w",
		},
	}); err != nil {
		t.Fatalf("不带 expected_update_time 应可保存: %v", err)
	}

	// 4) 文档同规则。
	doc, err := f.s.CreateDoc(ctx, f.admin.ID, f.admin.Type, dto.DocCreateReq{
		ProjectID: f.proj.ID, Title: "lock-doc", Content: "v1",
	})
	if err != nil {
		t.Fatalf("CreateDoc: %v", err)
	}
	loadedDoc, err := f.s.GetDoc(ctx, f.admin.ID, f.admin.Type, doc.ID)
	if err != nil {
		t.Fatalf("GetDoc: %v", err)
	}
	docStamp := loadedDoc.UpdateTime.Format(time.RFC3339Nano)
	if _, err := f.s.UpdateDoc(ctx, f.admin.ID, f.admin.Type, dto.DocUpdateReq{
		ID: doc.ID, Title: "lock-doc", Content: "v2", ExpectedUpdateTime: docStamp,
	}); err != nil {
		t.Fatalf("文档首次保存应成功: %v", err)
	}
	_, err = f.s.UpdateDoc(ctx, f.admin.ID, f.admin.Type, dto.DocUpdateReq{
		ID: doc.ID, Title: "lock-doc", Content: "v3", ExpectedUpdateTime: docStamp,
	})
	if !errors.As(err, &be) || be.Code != errcode.CodeFailed {
		t.Fatalf("文档旧版本保存应报冲突，got %v", err)
	}
	if _, err := f.s.UpdateDoc(ctx, f.admin.ID, f.admin.Type, dto.DocUpdateReq{
		ID: doc.ID, Title: "lock-doc", Content: "v4",
	}); err != nil {
		t.Fatalf("文档不带 expected_update_time 应可保存: %v", err)
	}
}
