package handler

// 审核 H-18 回归：
//   - 上传时客户端指定的 multipart Content-Type 不能入库；
//   - 下载只按扩展名映射 Content-Type + nosniff，PNG+HTML polyglot 不会被浏览器当 HTML 执行。

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"apidoc/internal/pkg/fileutil"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestFileHashServesSafeContentType 上传 text/html 的 .png，下载必须仍是 image/png。
func TestFileHashServesSafeContentType(t *testing.T) {
	t.Parallel()
	h, engine, svc := newTestHandler(t)
	h.cfg.Upload.Dir = t.TempDir()
	h.cfg.Upload.MaxMB = 5
	h.cfg.Upload.AllowExt = fileutil.DefaultAllowExt
	engine = gin.New()
	engine.POST("/file/upload", h.UploadImg)
	engine.GET("/file/:hash", h.GetFileByHash)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="evil.png"`)
	hdr.Set("Content-Type", "text/html") // 客户端可控，必须被忽略
	part, err := w.CreatePart(hdr)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	// 真实 PNG 签名 + 后面附加 HTML：嗅探通过，浏览器可执行 polyglot。
	if _, err := part.Write([]byte("\x89PNG\r\n\x1a\n<html><script>alert(1)</script></html>")); err != nil {
		t.Fatalf("write part: %v", err)
	}
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/file/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	var body response.Body
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != 200 {
		t.Fatalf("上传失败: code=%d msg=%s err=%v", body.Code, body.Msg, err)
	}
	data, _ := body.Data.(map[string]any)
	hash, _ := data["hash"].(string)
	if hash == "" {
		t.Fatalf("上传未返回 hash: %+v", body.Data)
	}
	f, err := svc.GetFileByHash(req.Context(), hash)
	if err != nil || f == nil {
		t.Fatalf("GetFileByHash: %v", err)
	}
	if f.Mime != "image/png" {
		t.Fatalf("入库 MIME 应按扩展名归一为 image/png，got %q", f.Mime)
	}

	get := httptest.NewRequest(http.MethodGet, "/file/"+hash, nil)
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, get)
	if w2.Code != http.StatusOK {
		t.Fatalf("下载失败: %d", w2.Code)
	}
	if ct := w2.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("下载 Content-Type 应为 image/png，got %q", ct)
	}
	if w2.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("下载必须带 X-Content-Type-Options: nosniff")
	}
	if !strings.HasPrefix(w2.Header().Get("Content-Disposition"), "inline") {
		t.Fatalf("下载应 inline 展示，got %q", w2.Header().Get("Content-Disposition"))
	}
	if !bytes.HasPrefix(w2.Body.Bytes(), []byte("\x89PNG")) {
		t.Fatal("下载内容应保持原文件")
	}
}
