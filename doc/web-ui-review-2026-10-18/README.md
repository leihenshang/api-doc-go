# api-doc-go Web 端 UI 评审

> 评审日期：2026-10-18 · 对象：`api-doc-go/web`（Vue3 + Naive UI 管理端，对 `/api/v1`）
> 结论口径：**页面简洁明了 / 重要信息容易读 / 开箱即用**
> 附件：`screenshots/`（12 张实机截图）、`serve-dist.mjs`（本地起生产产物 + /api 代理）

---

## 0. 一句话结论

信息架构是对的（顶栏 + 页签 + 项目内六分区），中文文案、空态、成员/日志/分享这些"管理端该有的"都在；
但**两个功能性 P0 挡住了它**：带登录态刷新即白屏、OpenAPI 导入丢参数。
UI 层的主要问题是**留白与密度失衡**（卡片固定 250px、空态动辄 150px、三栏信息条排错列）与**主色对比度不足**。

| 维度 | 评分 | 主要扣分项 |
| --- | --- | --- |
| 简洁明了 | 6.5 / 10 | 项目卡片固定宽度导致大面积空白；tab 与 H1 重复；接口详情三个空区块 |
| 重要信息容易读 | 6.0 / 10 | 主色白字 3.38:1；信息条 4 列放 5 项；响应样例单行压缩 |
| 开箱即用 | 6.5 / 10 | 无"示例项目/演示数据"；注册/登录页无默认账号提示；空态无下一步动作 |
| 稳定性 | **8.0 / 10**（修复后） | 原 4.0：刷新白屏、导入丢参数——**两个 P0 已修**，见 §2 |

---

## 1. 评审方式

```bash
# 后端（SQLite + 独立库目录，端口 2028）
cd server && go build -o ../.tmp/ui-review-web/apidoc-server ./cmd/server
DB_DRIVER=sqlite DB_DSN=$PWD/apidoc.db PORT=2028 JWT_SECRET=... ../apidoc-server

# 前端（生产构建 + 本地静态服务 + /api 代理，避开 vite dev 的差异）
cd web && npx vite build --outDir dist-r6
PORT=4178 node doc/web-ui-review-2026-10-18/serve-dist.mjs dist-r6

# 截图：登录 → SPA 内导航逐页（整页刷新会白屏，见 P0-1）
```

样本：1 个项目（星云开放平台）· 3 个业务分组 + 1 个二级分组 · 11 个接口（手工 8 个 + OpenAPI 导入 3 个）· 1 篇文档 · 1 条备忘 · 1 个公共属性 · 1 位成员 · 11 条操作日志。

---

## 2. 两个 P0（已修复）

### P0-1 带登录态刷新 / 深链接进入受保护页 → 白屏 ✅ 已修（`23c5706`）

**根因：ESM 模块图死锁，不是渲染问题。**

- `/` 路由的外壳 `AppLayout` 是 `() => import(...)` 懒加载；
- 打包后 `AppLayout-*.js` **反向静态 import 了入口 chunk**（`index-*.js`），因为
  `AppLayout → api/auth → api/http → router`，而 router/main 都在入口 chunk 里；
- 首次导航在入口 chunk **求值期间**发起对 AppLayout 的 `import()`，而该 chunk 要等入口 chunk
  求值完成 → 互相等待，`import()` 永不 resolve。

**排查证据链**（记录一下，这类问题没有任何报错，只能靠排除法）：
- `vite dev` 与生产构建产物都复现 → 不是构建/服务问题；
- `AppLayout-*.js` 等 chunk 全部 200 且 MIME 正确 → 不是资源加载问题；
- `pageerror` / `unhandledrejection` / Vue `errorHandler` / `router.onError` **一条都没有**；
- 未登录访问同一 URL 能正常跳登录页（`LoginView` 不在环里）→ 与守卫无关；
- `__vue_app__` 存在、`#app` 只剩空 `n-config-provider`、**零业务请求**（`onMounted` 未执行）
  → 组件没被挂载；
- 在 `beforeEach` 里显式 `await import('@/views/AppLayout.vue')` → **永不 settle**，直接锁定。

**修复（两处，缺一不可）**：
1. `AppLayout` 改**静态 import**（外壳与入口同 chunk，首次导航不再跨 chunk 动态加载）；
2. `api/http.ts` 断开对 `@/router` 的静态依赖：新增 `setUnauthorizedHandler`，由 `main.ts`
   注入「跳登录」，去掉 `router → 视图 → http → router` 的环。

**验证**（生产产物 + 静态服务器）：`/projects`、`/projects/1`、`/projects/1/apis`、`/users`、
`/properties` **整页刷新全部正常渲染**；未登录访问 `/projects/1` 仍正确跳 `/login`。
截图：`screenshots/13-apis-after-reload.png`（刷新后的接口页）。

> 副产品：`main.ts` 保留了 `app.config.errorHandler`、`router/index.ts` 保留了 `router.onError`，
> 下次再出现"URL 变了但页面空白"时控制台会直接给原因。

### P0-2 OpenAPI 导入只写入名称/方法/URL ✅ 已修（`09f29d3`）

**根因**：`service.ImportOpenAPI` 构造 `model.Api` 时只填了
`ApiName / HttpMethodType / URL / ProtocolType / Source`，`share/openapi` 明明已经解析出
`Params` 与 `Responses`，却没人写进 `http_request_*` / `http_return_*`。

**修复**（按 Web/客户端既有列格式，与 `utils/api-params.ts` 同口径）：
- `http_request_params`：query/path/cookie 参数 → `apiParamRow[]`（fieldName/type/required/description/example）；
- `http_request_header`：`in=header` → `apiHeaderRow[]`；
- `http_return_sample`：状态码最小的 2xx 且带 example 的响应原文；
- `http_return_params`：无 schema 时按该样例的**顶层字段**推导（排序稳定），并把 `http_return_type` 置 `object`；
- `description`：取 `operation.description`。

复用同包已有的 `apiParamRow`/`apiHeaderRow`/`jsonTypeName`/`jsonExample`，只补 `jsonExampleOrRaw`。

**验证**：单测 `TestImportOpenAPICarriesParamsAndSample`；端到端（overwrite 重导一份 4 接口的 spec）
后接口详情：请求参数 3 条带示例/说明、返回参数 2 条（code=integer / data=object）、响应样例格式化显示。
截图：`screenshots/14-api-detail-imported.png`。

> 已知残留：参数「类型」列目前为空——`share/openapi` 的 `Param` 没有解析 `schema.type`，
> 补它要给共享库加字段并升版本，留作后续（不影响可用性）。

## 3. 页面观察与 UI 问题清单

### 3.1 项目列表

![项目列表](screenshots/02-projects.png)

- **UI-01（P1，已修）项目少时右侧大片空白**（实测 4 列 × 338px，1 个项目占 1 列 → 空出 ~1000px）。→ 在卡片区末尾加「＋ 新增项目」虚线磁贴：空白有了落点，同时多了一个新建入口。截图 `screenshots/15-projects-tile.png`。
- **UI-02（P2）页签「项目列表」与页面 H1「项目列表」重复**。全局页签已表达位置，H1 可改为项目数摘要（"3 个项目 · 最近更新 …"）或直接省略。
- **UI-03（P2）卡片操作是三个同权重文字链**（详情/编辑/删除），删除虽为红色但没有二次确认的视觉提示。

### 3.2 项目概况

![项目概况](screenshots/03-project-overview.png)

- **UI-04（P1，已修）信息条 4 列却放了 5 项**，第 5 项（API 接口数）掉到第二行只占 1/4 宽、没有右边框，看起来像表格破了。→ 改 5 列 + `<1180px` 退 3 列。
- **UI-05（P1，已修）标题用主题色**（22px / 600 / `#18a058`）：与"可点击链接"同色，语义上像链接。→ 标题改正文色，主题绿只留给动作。截图 `screenshots/16-overview-title.png`。
- **UI-06（P2，已修）分享链接空态占 150px 高** → 换成一行虚线框 + 「生成一条」按钮（`screenshots/18-overview-members.png`）。
- **UI-07（P2，已修）成员卡片信息稀**：三行（名字 / 角色 / 账号）压成两行（名字 / 「负责人 · admin」），padding 12→10。

### 3.3 接口列表 / 详情

![接口详情](screenshots/05-api-detail.png)

- **UI-08（P1，已修）三个"暂无"空区块各占 100px**，新建接口时首屏大半是空白且**没有下手的地方**。→ 空态压到一行高（padding 18→10），框内加「编辑」按钮（`ApiParamTable` 新增 `edit` 事件，只切编辑态、不直接塞空行）。
- **UI-09（已核实非缺陷）**：`JsonBlock` 本身就会 `JSON.parse` + 两空格美化；早先截图里的"单行"是样本数据被双重转义成字符串导致解析失败（`P0-2` 的连带现象）。P0-2 修好后样例已正常多行展示（见 `14-api-detail-imported.png`）。
- **UI-10（已核实非缺陷）**：那 4 个灰方块是右侧「悬浮大纲」的段落格子（收起态），自带 `title` 与点击跳段，悬停会展开成段落列表，不是无标签按钮。
- **UI-11（P2，已修）内容区无最大宽度**：`.req-sec` 加 `max-width: 960px`，长 URL 与长描述不再横跨 1100px（`screenshots/19-api-detail-narrow.png`）。
- **UI-12（P2）左侧树重名接口无区分**（导入后出现两个「获取用户详情」），只能靠路径 hover 区分。

### 3.4 变量 / 备忘 / 用户 / 属性

![变量](screenshots/09-variables.png)

- **UI-13（P2）变量页顶部绿条一句话塞两种语义**：「环境 暂无环境，项目变量将只能使用「通用」值 + 新建环境」——现状 + 后果 + 动作挤在一行，建议拆成"提示 + 主按钮"两段。
- **UI-14（P2）空态只有"暂无变量"**，没有"新建变量"的引导（按钮在右上角，视线跳很远）。

### 3.5 登录页

![登录](screenshots/01-login.png)

- **UI-15（P2）卡片 310px 偏窄**，标题与副标题字号接近（14/12），层级弱；无「忘记密码」与默认账号提示（内网工具常见 admin/admin123，首次使用者会卡住）。

---

## 4. 本轮已落地的改动

| 项 | 改动 | 文件 |
| --- | --- | --- |
| P0-1 辅助 | 加 `app.config.errorHandler` + `router.onError` 日志（白屏不再静默） | `src/main.ts`、`src/router/index.ts` |
| UI-04 | 项目概况信息条 4 列 → **5 列**（窄屏 3 列） | `views/project/ProjectOverviewView.vue` |
| UI-16 | 主色 `#18a058` → **`#0f7a41`**（白字 3.38:1 → 5.41:1，hover 4.92:1） | `src/App.vue` |
| — | `.gitignore` 增加 `dist-*/`（本地多地构建产物不误提交） | `web/.gitignore` |
| **P0-1** | AppLayout 静态 import + `setUnauthorizedHandler` 断开 http→router 环（**白屏修复**） | `router/index.ts`、`api/http.ts`、`main.ts` |
| **P0-2** | OpenAPI 导入补齐 params/header/return_sample/return_params/description | `service/extra.go` + 回归测试 |
| UI-01 | 项目列表加「＋ 新增项目」磁贴 | `ProjectListView.vue` |
| UI-05 | 概况页标题改正文色 | `ProjectOverviewView.vue` |
| UI-08 | 参数表空态压高 + 「＋ 添加一行」入口 | `ApiParamTable.vue`、`ApiDetailView.vue` |
| UI-06 | 分享链接空态压成一行 + 「生成一条」 | `ShareLinksPanel.vue` |
| UI-07 | 成员卡片三行压两行 | `ProjectOverviewView.vue` |
| UI-11 | 接口详情正文限宽 960px | `ApiDetailView.vue` |

**未做**：UI-02（tab 与 H1 重复，属信息架构取舍）、UI-12（树里同名接口无区分）、
UI-13/14（变量页提示文案与空态引导）、UI-15（登录页默认账号提示——涉及安全策略，不宜默认展示）。

---

## 5. 建议的修复顺序

1. ~~P0-1 刷新白屏~~ ✅ 已修；
2. ~~P0-2 OpenAPI 导入丢参数~~ ✅ 已修；
3. UI-01 项目卡片自适应、UI-08 空区块压缩 + 就地添加入口、UI-09 响应样例统一格式化；
4. UI-05 标题色、UI-11 内容区最大宽度、UI-02 标题去重；
5. 其余 P2：空态引导、成员卡片、登录页提示、树重名。

---

## 6. 与客户端评审的关系

同一天的两份评审（`api-doc-go-client/doc/ui-review-2026-10-18`）暴露了同一个根因：**主色既当品牌色又当动作色**（白字不足 4.5:1）。两端现在都改成了 `#0f7a41`，建议后续把这条写进各自的设计令牌说明，避免再回退。
