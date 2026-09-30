# API-DOC（服务端）

轻量化接口文档协作工具，支持 Markdown 编写、按项目 / 分组管理接口与文档。本仓库为 **Go + Vue3** 重构版；旧的 PHP（`backend/`）、Vue2（`front/`）仅作历史参考，已不再维护，勿在其上开发。

## 功能特性

- 接口 / 文档的 Markdown 编辑与实时预览
- 按项目、分组组织接口与文档，支持回收站（批量还原 / 彻底删除）
- 环境变量（含敏感值掩码）与请求级变量替换
- 导入 / 导出：OpenAPI 3.0、Postman、Bruno、Markdown、自包含 HTML
- 代码生成（curl / fetch / axios / go / python）、内置 Mock、双向同步（PAT 鉴权 + 内容寻址）
- 中英双语界面

## 技术栈

- 后端：Go 1.26 + gin + gorm + glebarez/sqlite（纯 Go）/ gorm.io/driver/postgres
- 前端：Vue 3 + TypeScript + Vite + Pinia + Vue Router 4 + Naive UI + axios
- 统一响应：`{ code, msg, data }`，HTTP 恒 200

## 目录结构

```
server/    Go 后端（cmd/server 入口；internal/{config,database,model,dto,repository,service,handler,router,middleware,pkg}）
web/       Vue3 前端（src/{api,router,stores,views,components,types,utils}）
doc/       设计与规范文档（项目开发规范.md 为权威约定）
scripts/   check.sh 工程门禁（gofmt/build/vet/test/vue-tsc/eslint/vite build）
```

## 快速开始

### 后端

```bash
cd server
# 默认加载 bin/env.test：端口 2027、SQLite（data/apidoc.db）
./bin/run.sh start        # 启动；stop / restart / status / build 同理
# 或直接运行源码
go run ./cmd/server
```

默认超级管理员：`admin / admin123`（首次启动自动 seed；可用 `ADMIN_NAME` / `ADMIN_PASSWORD` 覆盖；强制重置 `go run ./cmd/seed`）。

### 前端

```bash
cd web
npm install
npm run dev               # http://localhost:5173，代理 /api -> 后端
npm run build             # vue-tsc 类型检查 + 构建
```

## 环境变量（后端，节选）

| 变量 | 说明 | 默认 |
|---|---|---|
| `PORT` | 监听端口 | 8080 |
| `GIN_MODE` | debug / release | debug |
| `DB_DRIVER` | `sqlite` / `postgres` | sqlite |
| `DB_DSN` | sqlite 文件路径或 postgres 连接串 | data/apidoc.db |
| `JWT_SECRET` / `JWT_EXPIRE_HOURS` | 鉴权 | 24 |
| `UPLOAD_DIR` | 上传目录 | - |
| `ADMIN_NAME` / `ADMIN_PASSWORD` | seed 超管 | admin / admin123 |

完整变量见 `server/internal/config`。

## 校验

```bash
cd server && go build ./... && go vet ./... && go test ./...
cd web    && npm run check          # type-check + lint + build
```

根目录 `scripts/check.sh` 跑全套门禁（与 CI 同源）。

## 共享模块

变量解析、集合格式、代码生成等两端共用的规则集中在 [api-doc-go-share](https://github.com/leihenshang/api-doc-go-share)，本仓库 `require v0.3.0`。

## 许可证

MIT（见 [LICENSE](./LICENSE)）。
