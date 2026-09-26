package fileutil

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
)

// pngHead 生成一个真实 PNG 的文件头字节（可能不足 512 字节，SniffAllowed 需容忍短输入）。
func pngHead(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("生成 PNG: %v", err)
	}
	return buf.Bytes()
}

// TestParseAllowExt 覆盖白名单解析：补点、小写、去重、默认值兜底。
func TestParseAllowExt(t *testing.T) {
	if got := ParseAllowExt(""); len(got) != len(DefaultAllowExt) {
		t.Fatalf("空配置应回退默认白名单，got %v", got)
	}
	got := ParseAllowExt(" PNG , .Jpg ,png,, svg ")
	want := []string{".png", ".jpg", ".svg"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("解析结果不符，got %v", got)
	}
	if got := ParseAllowExt(" , "); len(got) != len(DefaultAllowExt) {
		t.Fatalf("全空配置应回退默认白名单，got %v", got)
	}
}

// TestNormalizeAndAllowed 覆盖扩展名归一化与白名单判断。
func TestNormalizeAndAllowed(t *testing.T) {
	cases := map[string]string{
		"a.PNG": ".png", "dir/b.jpeg": ".jpeg", "noext": "", "trailing.": "", ".hidden": ".hidden",
	}
	for in, want := range cases {
		if got := NormalizeExt(in); got != want {
			t.Fatalf("NormalizeExt(%q) = %q, want %q", in, got, want)
		}
	}
	if !Allowed(".png", DefaultAllowExt) || Allowed(".html", DefaultAllowExt) || Allowed("", DefaultAllowExt) {
		t.Fatal("白名单判断不符")
	}
	// svg 默认被排除（公开目录下的 XSS 风险）
	if Allowed(".svg", DefaultAllowExt) {
		t.Fatal("默认白名单不应包含 svg")
	}
}

// TestSniffAllowed 覆盖内容嗅探：真实 PNG 通过，改名后的 HTML/脚本被拒。
func TestSniffAllowed(t *testing.T) {
	if !SniffAllowed(pngHead(t), ".png") {
		t.Fatal("真实 PNG 应通过嗅探")
	}
	html := []byte("<!DOCTYPE html><html><script>alert(1)</script></html>")
	if SniffAllowed(html, ".png") {
		t.Fatal("HTML 内容改名为 .png 应被拒绝")
	}
	if SniffAllowed([]byte("plain text"), ".pdf") {
		t.Fatal("文本内容不应被当作 PDF")
	}
	if SniffAllowed(pngHead(t), ".svg") {
		t.Fatal("白名单外扩展名一律拒绝")
	}
}

// TestRandomNameAndContentType 覆盖随机命名与响应类型映射。
func TestRandomNameAndContentType(t *testing.T) {
	a, err := RandomName(".png")
	if err != nil {
		t.Fatalf("RandomName: %v", err)
	}
	b, _ := RandomName(".png")
	if len(a) != 32+len(".png") || a == b || !strings.HasSuffix(a, ".png") {
		t.Fatalf("随机命名不符：%q / %q", a, b)
	}
	if ContentTypeOf(".png") != "image/png" || ContentTypeOf(".svg") != "application/octet-stream" {
		t.Fatal("Content-Type 映射不符（未知类型必须回退 octet-stream）")
	}
}

// TestSign 覆盖访问签名：空密钥不启用、同输入稳定、不同文件不同签名。
func TestSign(t *testing.T) {
	if Sign("a.png", "") != "" {
		t.Fatal("空密钥应返回空签名（表示不启用）")
	}
	s1 := Sign("a.png", "k1")
	if s1 == "" || s1 != Sign("a.png", "k1") || s1 == Sign("b.png", "k1") || s1 == Sign("a.png", "k2") {
		t.Fatalf("签名计算不符：%q", s1)
	}
	if len(s1) != 32 {
		t.Fatalf("签名长度应为 32，got %d", len(s1))
	}
}
