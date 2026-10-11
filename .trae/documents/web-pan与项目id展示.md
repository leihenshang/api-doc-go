# Web 补全客户端对接配置（PAT 管理 / client_write_enabled / 项目 ID 展示）

## Context

桌面客户端（api-doc-go-client）通过「PAT + 项目 ID」绑定服务端项目并做增量同步，服务端还提供 `project.client_write_enabled`（S14，一键禁止客户端写）开关。目前：
- PAT 接口（`POST/GET/DELETE /api/user/token`）已实现但 **Web 前端零入口**，用户只能 curl 创建；
- 项目 ID（`project` 表自增主键 `uint64`）**界面从不展示**，客户端绑定面板要手动填，用户不知道在哪看；
- `client_write_enabled` 后端有字段但 **DTO 不透传**，Web 无法配置。

目标：把这三块补到 Web 界面。已与用户确认：PAT 入口 = **个人中心页面**；项目 ID 展示 = **项目概况页 + 项目列表卡片**；写入开关 = **编辑项目弹窗**。

## 项目 ID 是什么？客户端要填吗？

- **项目 ID** = `project` 表主键 `ID uint64`（自增），服务端所有项目级接口（接口/文档/变量/同步/分享）都以 `project_id` 定位项目。
- **客户端必须填**：客户端是文件化本地集合，绑定前没有服务端项目列表可拉取，`SyncDialog` 里用户手动输入 `projectId`（存 `.sync.json`）。没有它，同步请求（`/api/sync/*`）无法定位项目。
- **Web 展示位置**：项目概况页信息列表（加一行「项目 ID」+ 复制按钮）、项目列表卡片（`#id` 徽标）。

## 改动清单

### A. 后端（最小透传 + 文档）
1. `server/internal/dto/dto.go` `ProjectUpdateReq` 加 `ClientWriteEnabled *bool \`json:"client_write_enabled"\``（指针区分未传，避免误关）。
2. `server/internal/service/project.go` `UpdateProject` 透传：`if req.ClientWriteEnabled != nil { p.ClientWriteEnabled = req.ClientWriteEnabled }`。
3. `server/doc/api_docs/project/project_update.yml` 参数表补 `client_write_enabled`（`*bool`，可空）。

### B. 前端 api 层与类型
1. `web/src/types/entities.ts`：`Project` 加 `client_write_enabled?: boolean | null`；新增 `UserToken { id; name; scopes; expire_at; last_used; revoked; create_time }` 与 `CreateTokenResp { token; info: UserToken }`。
2. `web/src/api/auth.ts` 加三个函数（沿用 `get/post/put/del<T>` 解包模式，见 [auth.ts](file:///d:/my-project/api-doc-go-github/api-doc-go/web/src/api/auth.ts)）：
   - `createToken(payload: { name: string; scopes?: string; days?: number })` → `POST /user/token`
   - `listTokens()` → `GET /user/token`
   - `revokeToken(id: number)` → `del('/user/token', { id })`
   - 注：服务端 `scopes` 不校验（可空，前端默认填 `sync`）。

### C. PAT 管理：个人中心（用户决策）
1. 新建 `web/src/components/TokenPanel.vue`：令牌列表（`n-table`：名称/作用域/有效期/状态/「失效」按钮，失效前 `dialog.warning` 确认）+ 创建表单（名称/作用域/有效天数，`days` 默认 0=不过期）+ **创建成功后展示明文**（`n-alert` 或结果区 + 复制按钮，明文仅返回一次）。参照 `ProfileModal.vue` 的 message/ErrCode 判错模式与 `ProjectOverviewView` 的表格样式。
2. `web/src/views/user/MyCenterView.vue` 嵌入 `<token-panel />` 卡片。
3. 给个人中心加导航入口（目前 `myCenter` 路由存在但无入口）：`web/src/components/UserMenu.vue` 菜单加「个人中心」项（key=`center`，icon 用 `PersonOutline`）→ `router.push({ name: 'myCenter' })`。

### D. 项目 ID 展示（用户决策：概况页 + 列表卡片）
1. `web/src/views/project/ProjectOverviewView.vue` 信息列表（`<ul class="info">`）加「项目 ID」li：显示 `projectId` + 复制按钮（`navigator.clipboard.writeText` + `message.success(t('common.copied'))`）。
2. `web/src/views/ProjectListView.vue` 卡片 `.pcard-badges` 或 `.pcard-ft` 加 `#{{ p.id }}` 徽标（muted），点击复制。

### E. client_write_enabled 开关（用户决策：编辑弹窗）
1. `web/src/views/project/ProjectOverviewView.vue` 编辑弹窗 `form` 加 `client_write_enabled` 字段（`n-switch`），`openEdit` 时取 `project.current?.client_write_enabled ?? true`，`submitEdit` 的 `updateProject` payload 带上该字段。
2. `web/src/views/ProjectListView.vue` 编辑弹窗同样加开关（两处编辑表单保持一致）。

### F. i18n（zh-CN / en-US key 一一对应，改完跑 `npm run i18n:check`）
- `common`：`copy` 复制、`copied` 已复制
- `user`：`center` 个人中心、`tokenTitle` 访问令牌、`tokenSub`、`tokenName`/`tokenScopes`/`tokenDays`/`tokenDaysHint`、`tokenCreate`、`tokenCreateSuccess`（仅展示一次）、`tokenEmpty`、`tokenRevoke`/`tokenRevokeConfirm`
- `project`：`id` 项目 ID、`idHint`（用于桌面客户端关联本项目）、`clientWrite` 允许客户端写入、`clientWriteHint`（关闭后客户端只能拉取、无法推送）

### G. 复用与约束
- 弹窗/判错模式：`ProfileModal.vue`（`defineModel('show')`、`message + ErrCode.Success`）
- 表格/卡片样式：`ProjectOverviewView` 的 `.info/.members`、`ProjectListView` 的 `.pcard`
- 语言规则：文案一律 `t()`，非组件处用 `translate()`；新增 key 两包同步

## 验证

1. 后端（server/）：`go build ./... && go vet ./...`（`go test` 的 Windows 临时库清理失败为既有环境问题，忽略）。
2. 前端（web/）：`npm run i18n:check`、`npx vue-tsc --noEmit`、`npm run lint`、`npm run build`。
3. 端到端手动：登录 → 头像菜单「个人中心」→ 创建 PAT → 复制明文 → 项目概况页看到「项目 ID」并复制 → 用 PAT + 项目 ID 在桌面客户端 SyncDialog 绑定 → 编辑项目关闭「允许客户端写入」→ 客户端再同步确认被 `rejected.readonly` 拒绝。
