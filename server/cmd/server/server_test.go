package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestNewHTTPServerTimeouts 校验服务端四大超时均已配置（审核 H-12：防 Slowloris 与慢请求占用）。
func TestNewHTTPServerTimeouts(t *testing.T) {
	srv := newHTTPServer(":0", http.NewServeMux())
	if srv.ReadHeaderTimeout <= 0 || srv.ReadTimeout <= 0 || srv.WriteTimeout <= 0 || srv.IdleTimeout <= 0 {
		t.Fatalf("timeouts not configured: %+v", srv)
	}
	if srv.ReadHeaderTimeout != readHeaderTimeout || srv.IdleTimeout != idleTimeout {
		t.Fatalf("timeouts mismatch: %+v", srv)
	}
}

// TestServeShutdownWaitsInFlight 校验 ctx 取消后进入优雅关闭：在途请求完成，serve 返回 nil。
func TestServeShutdownWaitsInFlight(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	started := make(chan struct{})
	srv := newHTTPServer("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(120 * time.Millisecond)
		_, _ = w.Write([]byte("ok"))
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, srv, ln) }()

	respCh := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			respCh <- err
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			respCh <- err
			return
		}
		if resp.StatusCode != http.StatusOK || string(body) != "ok" {
			respCh <- fmt.Errorf("status=%d body=%q", resp.StatusCode, body)
			return
		}
		respCh <- nil
	}()

	<-started
	cancel() // 在途请求期间取消：Shutdown 必须等待该请求完成

	if err := <-done; err != nil {
		t.Fatalf("serve returned error: %v", err)
	}
	if err := <-respCh; err != nil {
		t.Fatalf("in-flight request failed: %v", err)
	}
}

// TestServeReturnsListenError 校验监听器不可用时直接返回错误（不静默退出）。
func TestServeReturnsListenError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	if err := serve(context.Background(), newHTTPServer("", http.NewServeMux()), ln); err == nil {
		t.Fatal("expected error for closed listener")
	}
}
