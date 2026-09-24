# AGENTS.md

面向 AI 编码助手的项目指南。开始任何改动前先读本文件与 `doc/项目开发规范.md`。

## 项目概览
API-DOC：轻量化接口文档协作工具，支持 Markdown、按项目/分组管理接口与文档。
本仓库为 **Go + Vue3 + SQLite(默认)/PostgreSQL** 重构版；旧的 PHP(`backend/`)、Vue2(`front/`) 仅作参考，勿在其上开发。

技术栈：
- 后端：Go 1.26 + gin + gorm + glebarez/sqlite(纯 Go) + gorm.io/driver/postgres
- 前端：Vue 3 + TypeScript + Vite + Pinia + Vue Router 4 + Naive UI + axios
- 统一响应：`{ code, msg, data }`，HTTP 恒 200

## 目录结构
```
server/           Go 后端
  cmd/server/       入口 main.go
  internal/
    config/         配置加载（env）
    database/       GORM 连接 + AutoMigrate
    model/          GORM 实体（表结构）
    dto/            请求/响应结构 + 校验 tag（按操作拆分）
    repository/     数据访问（仅 CRUD，无业务）
    service/        业务逻辑（事务、规则、编排 repo，对外暴露 interface）
    handler/        HTTP 层（解析请求→调 service→统一响应）
    router/         路由注册
    middleware/     auth、permission、cors 等
    pkg/            response、errcode、page、jwt、logger（无业务依赖）
  migrations/       生产迁移（sqlite/ 与 postgres/ 两套）
web/              Vue3 前端
  src/{api,router,stores,composables,views,components,types,utils}
doc/              设计与规范文档
  项目开发规范.md    ← 权威约定（分层、命名、规模、DB）
  功能清单.md        现有功能/接口来源
  api_docs/          接口文档 yml（一接口一文件，按模块分组）
  项目进度.md         进度与待办
database/  sql/   旧库脚本，仅作字段/关系参考
```
依赖方向（单向）：`handler → service → repository → model`；`dto` 横切于边界；`pkg` 被复用不反依赖。

## 常用命令
后端（在 `server/`，服务由 `bin/run.sh` 控制，默认加载 `bin/env.test` → **端口 2027、SQLite**）：
```bash
./bin/run.sh start|stop|restart|status|build   # 启停/状态/仅构建
go build ./... && go vet ./... && go test ./...  # 开发期校验
```
前端（在 `web/`）：
```bash
npm install
npm run dev            # Vite 前台，:5173，代理 /api、/uploads → VITE_PROXY_TARGET（默认 2027）
npm run dev:start      # 后台启动 dev（.dev.pid/.dev.log）；dev:stop / dev:status 配套
npm run build          # vue-tsc 类型检查 + 构建
npx vue-tsc --noEmit   # 仅类型检查（提交前必过）
```
测试账号：启动时自动种子超管 `admin / admin123`（仅当不存在时创建，可用 `ADMIN_NAME/ADMIN_PASSWORD` 覆盖；强制重置用 `go run ./cmd/seed`）。

## 环境变量（后端）
- `PORT`（默认 8080）、`GIN_MODE`（debug/release）
- `DB_DRIVER`：`sqlite`(默认) 或 `postgres`；`DB_DSN`：sqlite 为文件路径（默认 `data/apidoc.db`），postgres 为连接串
- `JWT_SECRET`、`JWT_EXPIRE_HOURS`（默认 24）
- `SECRET_ENCRYPT_KEY`：外部库口令 AES-GCM 加密主密钥（生产必须修改）
- `UPLOAD_DIR`、`BACKUP_ENABLED/BACKUP_INTERVAL_MIN/BACKUP_DIR`、`LOG_RETENTION_DAYS`（默认 70）、`ADMIN_NAME/ADMIN_PASSWORD`（seed 用）

## 编码约定（必须遵守，详见 规范 §4）
- 函数 ≤ 50 行、嵌套 ≤ 3 层；超出则拆小函数（命名仍具体）。
- Go 位置参数 ≤ 4 个，超出改用参数结构体/Options；前端函数参数 ≤ 3，选项用对象。
- Go 签名：`func X(ctx context.Context, req *dto.XReq) (*dto.XResp, error)`；`ctx` 为首参；返回 `(result, error)`，不得吞错。
- 错误用 `fmt.Errorf("...: %w", err)` 包装；对外错误经 `errcode` 映射。
- 单一职责、命名即语义；魔法值用常量/枚举；公共函数写用途/参数/返回/错误注释。
- 分层职责不可越界：handler 不写业务与 SQL；service 管事务与规则并暴露 interface；repository 只做数据访问；model 只有表结构、dto 只有契约。
- 前端全量 TypeScript（`<script setup lang="ts">`），禁用隐式 any；类型契约在 `src/types/` 手工维护，**不做自动生成**。
- 前端布局：`AppLayout` = 顶栏（品牌 `RepoMark` 仓库图标 + 文案 `API-DOC-GO`，与 `document.title`/`index.html` 保持一致；右侧为**通知铃铛** + **头像悬停弹个人中心（含退出）** + **项目切换 `n-select`（顶栏正中）**）+ **横向 tab 栏（居中）** + 面包屑栏（左侧圆形返回按钮 `.back-btn`，**右侧 `#crumb-actions` 供页面 `<teleport defer>` 注入导入/导出/搜索**，首页项目列表不渲染面包屑栏）。tab/面包屑/回收站逻辑集中在 **`composables/projectNav.ts`**（导出 `tabs/activeTab/trashActive/onTab/switchProject/crumbs`）：项目内 tab = 项目概况/API接口/项目文档/字段映射/Bug 管理/数据字典/操作日志，全局 tab = 项目列表/用户管理/公共属性；**回收站 tab 右对齐且按当前分区**（`apis`/`docs` → `?trash=1`，默认 API）。**默认进入项目即 API接口**（`ProjectListView` 链接与 `switchProject` 指向 `projectApis`）。**接口/文档为工作台布局**：`ApiListView`/`DocListView` = 左 `SectionTree.vue`（分组节点 + 其下**接口/文档叶子**，顶部**树上搜索**过滤叶子，分组仍可悬停新建子分组/重命名/删除，最多 7 级；数据源 `stores/project.ts` 分组 + `listApis/listDocs` 全量叶子，`utils/group-tree.ts` 按 `p_id` 组树）+ 右侧内容（`?api=<id>`/`?doc=<id>` 内嵌 `ApiDetailView`/`DocDetailView`（props `embedded`，支持 `?edit=1`）、`?new=1` 新建接口全屏、`?trash=1` 回收站表格、**无选中时 `EmptyHint` 空态（大号浅色图标 + 引导提示，接口页 `SendOutline`/文档页 `DocumentTextOutline`）**）。原 `apiDetail`/`docDetail` 独立路由与 `GroupTree.vue` 已删除。项目内**一屏一路由**（`views/project/*View.vue`）。**接口页/文档页为 Postman 风格全出血两栏**：`AppLayout` 对 `projectApis`/`projectDocs` 取消 `.page-body` 内边距（`.page-body--flush`），`SectionTree` 为扁平集合面板（计数 + 新建分组图标 + 搜索；分组行 = 折叠箭头 + 文件夹图标，叶子行 = **彩色方法文字**（`MethodTag` 的 `plain` 形态，Postman 配色）+ 名称）、右侧 `ApiDetailView` 为请求编辑器（名称栏 + **URL 栏**（方法下拉/彩色方法 + URL + 复制 URL + cURL + 编辑/保存主按钮）+ **下划线页签**（请求参数/请求头部/返回参数/响应样例/说明，带上标计数）+ 内容区）；接口页/文档页的整块内容（新建接口、回收站）用 `.pane-card` 留白，详情类内容用 `.pane`（也需给 `.n-spin-content` 撑满）。复用 `components/{SectionTree,ApiParamTable,ApiFieldsEditor,EmptyHint,MethodTag,JsonBlock,TablePager,RepoMark,UserAccountPanel,NotificationBell}.vue` 与 `styles/app.css` 的页面类（`.page/.page-hd/.toolbar/.row-actions/.table-scroll/.split/.split-tree/.split-main/.pane/.pane-card`）；**接口参数/请求头/返回参数表一律用 `ApiParamTable`**（`kind=request|response|header`，查看/编辑同版式；请求头经 `utils/api-params.ts` 归一化为参数行），**列表一律用 `TablePager` 常驻分页**（数据不足一页也显示「共 N 条」，勿写 `v-if="total > pageSize"`；后端不分页的接口在前端切片分页）；**带分页的列表卡片用 `.table-card` 纵向 flex 撑满**，`TablePager` 靠 `margin-top:auto` 贴卡片底；**新建长表单/回收站覆盖整个内容区（隐藏左树）**；**内容区背景须挂在滚动容器 `.content`（flex 撑满）上**；行内跳转用 `router-link`（不用 span 绑 click）；naive-ui 容器子节点必须配套子组件（`n-grid` → `n-grid-item`，`n-tabs` → `n-tab-pane`），否则被静默丢弃；⚠️ naive-ui `NCard` 内容层**实际类名是 `n-card-content`（单连字符）**，写 CSS 命中它时一并兼容 `n-card__content` 以防版本差异；⚠️ `gofmt -w` 只跑 `cmd internal`（`data/_modcache` 为只读 modcache，勿全仓格式化）。
- 后端查询参数命名以 `doc/api_docs/*.yml` 为准（如 `project_id`/`bug_id`）；handler 用 `idOrFallback(c, name)` 解析，勿只读 `id`。
- 接口文档 yml 与后端 DTO 字段/错误码/分页结构保持一致；接口变更同步更新 yml。

## 关键不变式
- 业务错误码沿用：`200 成功 / 22 参数错误 / 14 失败 / 34 未登录或过期 / 403 无权限`；`34` 前端会跳登录，勿改。
- 软删除统一 `is_deleted`（整型 0/1），列表查询默认过滤已删除。
- 数据库方言无关：业务 SQL 不写 sqlite/pg 专属语法；差异只在 `config`/`database`/`migrations`。
- 不做旧项目历史数据迁移；新库经 AutoMigrate/迁移全新建表 + 必要种子（如超管）。

## 当前进度
见 `doc/项目进度.md`。**P0–P11 全部完成**（后端全功能 + 前端全页面，含回收站/OpenAPI 导入导出/cURL/introspect/日志清理/属性管理/图床上传），单测 4 包、`api_docs/` 逐接口 yml 与 `migrations/` 生产 SQL 已备。前端已按 `project-overview/` 旧版截图**完成整站重设计**（侧导航 + 面包屑 + 表格化列表 + 项目内一页一路由）。剩余可选：OpenAPI YAML 文本导入、真实邮件/短信通道、golang-jwt/argon2 升级、CI 门禁。

## 注意
- 不要修改/删除 `backend/`、`front/`（留作参考），除非明确要求。
- 改动 Go 代码后跑 `gofmt -w . && go build ./...`；改动前端跑 `npx vue-tsc --noEmit`。
