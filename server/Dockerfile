# 多阶段构建：编译 Go 后端（CGO 关闭，纯 Go sqlite 驱动）
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/apidoc-server ./cmd/server
RUN CGO_ENABLED=0 go build -o /out/apidoc-seed ./cmd/seed

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/apidoc-server /app/apidoc-server
COPY --from=build /out/apidoc-seed /app/apidoc-seed
ENV PORT=8080 DB_DRIVER=sqlite DB_DSN=/app/data/apidoc.db UPLOAD_DIR=/app/data/uploads
EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["/app/apidoc-server"]
