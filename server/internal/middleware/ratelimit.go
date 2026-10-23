// Package middleware：按客户端 IP 的滑动窗口限流。
// 超限返回 **HTTP 200 + code 14**（与前端统一响应约定一致，前端只认 body.code；
// 返回 429 会走 axios 异常分支，调用方拿不到结构化消息），并带 Retry-After 头便于排障。
package middleware

import (
	"strconv"
	"sync"
	"time"

	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// rateLimiter 固定窗口计数限流器（窗口 1 分钟）。
type rateLimiter struct {
	mu      sync.Mutex
	hits    map[string]*window
	limit   int
	lastGC  time.Time
	nowFunc func() time.Time
	// maxEntries 键数量上限：X-Forwarded-For 是客户端可控的，来 100 万个不同 IP
	// 就能在 1 分钟窗口内把 map 撑到 100 万条（P1-11 的内存放大型 DoS）。
	// 到达上限后新 IP 全部折算到 overflowKey 这一条桶里——伪造源 IP 绕不过限流，
	// 正常用户也不会因为别人刷而立刻被误伤。
	maxEntries  int
	overflowKey string
}

const (
	defaultMaxRateEntries = 10000
	overflowRateKey       = "__overflow__"
)

type window struct {
	start time.Time
	count int
}

// newRateLimiter 构造限流器；limit <= 0 表示不限制（中间件直接放行）。
func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{
		hits: map[string]*window{}, limit: limit, nowFunc: time.Now,
		maxEntries: defaultMaxRateEntries, overflowKey: overflowRateKey,
	}
}

// allow 记录一次请求并返回是否放行。
func (l *rateLimiter) allow(key string) bool {
	if l.limit <= 0 {
		return true
	}
	now := l.nowFunc()
	l.mu.Lock()
	defer l.mu.Unlock()

	// 顺带清理过期窗口，避免 map 无限增长。
	if now.Sub(l.lastGC) > time.Minute {
		for k, w := range l.hits {
			if now.Sub(w.start) >= time.Minute {
				delete(l.hits, k)
			}
		}
		l.lastGC = now
		// 全是新窗口时 GC 清不掉，这里再按上限硬裁一次（多删几条比无限涨好）。
		if len(l.hits) > l.maxEntries {
			for k := range l.hits {
				if len(l.hits) <= l.maxEntries {
					break
				}
				if k != key {
					delete(l.hits, k)
				}
			}
		}
	}

	// 键数量到顶：新 IP 折算进溢出桶，既不让内存无界增长，也不给「换 IP 绕过限流」留口子。
	if _, exists := l.hits[key]; !exists && l.maxEntries > 0 && len(l.hits) >= l.maxEntries {
		key = l.overflowKey
	}

	w, ok := l.hits[key]
	if !ok || now.Sub(w.start) >= time.Minute {
		l.hits[key] = &window{start: now, count: 1}
		return true
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	return true
}

// retryAfter 返回距离窗口重置的秒数（>=1）。
func (l *rateLimiter) retryAfter(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.hits[key]
	if !ok {
		return 1
	}
	left := time.Minute - l.nowFunc().Sub(w.start)
	if left < time.Second {
		return 1
	}
	return int(left.Seconds())
}

// RateLimit 返回限流中间件：每 IP 每分钟 limit 次（limit <= 0 关闭）。
func RateLimit(limit int) gin.HandlerFunc {
	limiter := newRateLimiter(limit)
	return func(c *gin.Context) {
		if limiter.limit <= 0 {
			c.Next()
			return
		}
		key := ClientIP(c)
		if limiter.allow(key) {
			c.Next()
			return
		}
		c.Header("Retry-After", strconv.Itoa(limiter.retryAfter(key)))
		response.Fail(c, errcode.CodeFailed, "请求过于频繁，请稍后再试")
		c.Abort()
	}
}

// ClientIP 取客户端 IP：由 Gin 按 SetTrustedProxies 配置的可信代理链解析。
// 审核 H-19：未配置可信代理时只使用 TCP 对端地址，XFF/X-Real-IP 一概忽略；
// 否则客户端每次换一个头值就能绕开限流。
func ClientIP(c *gin.Context) string {
	return c.ClientIP()
}
