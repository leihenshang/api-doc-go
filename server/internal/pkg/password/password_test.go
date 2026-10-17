package password

// 口令哈希层测试（P0-5）：纯函数、成本最低，但它是账号体系的地基 ——
// 哈希算法或参数一改，若没有「同一口令两次生成不同、但都能校验通过」这类断言，
// 线上表现会是「密码突然全部失效」，且不可回滚。

import (
	"strings"
	"testing"
)

func TestGenerateVerifyRoundTrip(t *testing.T) {
	hash, err := Generate("p@ssw0rd-中文")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("应为 bcrypt 散列（$2 前缀），got %q", hash)
	}
	if hash == "p@ssw0rd-中文" {
		t.Fatal("散列不能等于明文")
	}
	ok, err := Verify("p@ssw0rd-中文", hash)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Fatal("正确口令应校验通过")
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	hash, err := Generate("secret")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// 不匹配是正常业务分支：返回 (false, nil)，不能报错，否则上层会当成系统故障。
	ok, err := Verify("Secret", hash)
	if err != nil {
		t.Fatalf("口令不匹配不应返回错误，got %v", err)
	}
	if ok {
		t.Fatal("大小写不同应校验失败")
	}
	// 空口令同样只是「不匹配」。
	if ok, err = Verify("", hash); err != nil || ok {
		t.Fatalf("空口令应返回 (false, nil)，got (%v, %v)", ok, err)
	}
}

func TestGenerateSalted(t *testing.T) {
	// 同一口令两次生成必须不同（随机盐），否则相同口令的用户在库里长得一样，
	// 泄露后即可批量识别「共用同一口令」的账号。
	h1, err := Generate("same")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	h2, err := Generate("same")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if h1 == h2 {
		t.Fatal("同一口令两次散列应不同（随机盐）")
	}
	// 但两者都必须能校验通过。
	for _, h := range []string{h1, h2} {
		if ok, err := Verify("same", h); err != nil || !ok {
			t.Fatalf("散列 %q 应校验通过，got (%v, %v)", h, ok, err)
		}
	}
}

func TestVerifyMalformedHash(t *testing.T) {
	// 格式非法是「系统故障」而不是「口令错」：必须返回错误，
	// 让上层能区分「密码错」与「库里数据坏了」。
	for _, bad := range []string{"", "not-a-hash", "$2a$10$short", "md5:abc"} {
		if _, err := Verify("whatever", bad); err == nil {
			t.Fatalf("非法散列 %q 应返回错误", bad)
		}
	}
}

func TestGenerateLongPassword(t *testing.T) {
	// bcrypt 只取前 72 字节且超长会报错：确认我们没有被静默截断到「不同口令等价」。
	long := strings.Repeat("a", 100)
	if _, err := Generate(long); err == nil {
		t.Fatal("超过 bcrypt 72 字节上限应报错，而不是静默截断")
	}
	// 刚好 72 字节仍可正常往返。
	edge := strings.Repeat("b", 72)
	hash, err := Generate(edge)
	if err != nil {
		t.Fatalf("72 字节口令应可哈希: %v", err)
	}
	if ok, err := Verify(edge, hash); err != nil || !ok {
		t.Fatalf("72 字节口令应校验通过，got (%v, %v)", ok, err)
	}
}
