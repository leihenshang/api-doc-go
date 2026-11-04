# API 文档服务端（单容器：Go 服务 + 内置托管 Web 前端）
#
# 构建：docker build -t apidoc-go:latest .
# 运行：docker run -d --name apidoc -p 8080:8080 \
#         -e ADMIN_PASSWORD='换成你的口令' -e JWT_SECRET='换成随机长串' \
#         -v apidoc-data:/app/data -v apidoc-uploads:/app/uploads apidoc-go:latest
#
# 说明：
#   - 数据库用 SQLite（纯 Go 驱动 modernc.org/sqlite，无 CGO），数据落在 /app/data 卷里；
#   - 前端构建产物拷到 /app/web，由服务端按 WEB_DIR 直接托管（单端口，不用再挂 nginx）；
#   - 上传文件落 /app/uploads 卷。

# ---------- 1) 前端构建 ----------
FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---------- 2) 服务端构建 ----------
FROM golang:1.26-alpine AS server
WORKDIR /src
# 先只拷依赖清单，命中缓存后再拷源码
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
# CGO 关掉：sqlite 用纯 Go 实现，产物是静态二进制，运行镜像可以用 alpine 甚至 scratch
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/apiserver ./cmd/server

# ---------- 3) 运行 ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -u 10001 app
WORKDIR /app

COPY --from=server /out/apiserver /app/apiserver
COPY --from=web /src/web/dist /app/web
RUN mkdir -p /app/data /app/uploads && chown -R app:app /app
USER app

ENV PORT=8080 \
    GIN_MODE=release \
    DB_DRIVER=sqlite \
    DB_DSN=/app/data/apidoc.db \
    WEB_DIR=/app/web \
    UPLOAD_DIR=/app/uploads \
    ADMIN_NAME=admin

# ADMIN_PASSWORD / JWT_SECRET 不设默认值：release 模式下服务端会拒绝启动，
# 强制部署者显式传入（避免把演示口令带进内网）。
EXPOSE 8080
VOLUME ["/app/data", "/app/uploads"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/apiserver"]
