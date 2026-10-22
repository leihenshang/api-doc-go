package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// HTTP 服务超时与优雅关闭参数（审核 H-12）：
//   - ReadHeaderTimeout 抵御 Slowloris（慢速发送请求头长期占用连接）；
//   - ReadTimeout / WriteTimeout 约束单次请求总时长，避免慢请求长期占用 goroutine；
//   - IdleTimeout 回收空闲长连接；
//   - shutdownTimeout 给在途请求收尾的窗口，超时后强制关闭。
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 120 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 15 * time.Second
)

// newHTTPServer 构造带完整超时配置的 HTTP 服务。
// 无超时的 http.Server 会暴露于 Slowloris，并让异常连接长期占用资源。
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// serve 在 ln 上服务请求，阻塞至 ctx 取消或服务出错。
// ctx 取消后进入优雅关闭：停止接收新连接，等待在途请求完成（最多 shutdownTimeout），
// 超时仍未收尾则返回错误。正常关闭返回 nil。
func serve(ctx context.Context, srv *http.Server, ln net.Listener) error {
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// 超时未收尾：强制断开剩余连接，避免进程悬挂。
		_ = srv.Close() // 忽略：Shutdown 已失败，Close 仅尽力回收
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
