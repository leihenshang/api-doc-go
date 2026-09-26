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
- `UPLOAD_DIR`、`BACKUP_ENABLED/BACKUP_INTERVAL_MIN/BACKUP_DIR`、`LOG_RETENTION_DAYS`（默认 70）、`ADMIN_NAME/ADMIN_PASSWORD`（seed 用）
- **本地数据不入库**（`server/.gitignore`）：SQLite 主库 `*.db` 及其旁路文件 `*.db-*`（`-wal/-shm/-journal`）、备份 `*.db.bak*`、`*.sqlite`/`*.sqlite3`/`*.sqlite*-*` 全部忽略；另忽略 `data/` 下的 `_modcache/`、`gocache/`、`tmp/`、`uploads/`、`backups/`（`BACKUP_DIR` 默认在此，内容为完整库副本）与 `data/apidoc-*` 二进制。**数据库与备份含明文数据（变量表可能存令牌），不要提交、不要贴进 issue**。`.gitignore` 不支持行尾注释（`#` 仅在行首有效），模式与说明必须分行写。保留项：`data/e2e.sh`、`data/genmigrate/`（源码）。

## 编码约定（必须遵守，详见 规范 §4）
- 函数 ≤ 50 行、嵌套 ≤ 3 层；超出则拆小函数（命名仍具体）。
- Go 位置参数 ≤ 4 个，超出改用参数结构体/Options；前端函数参数 ≤ 3，选项用对象。
- Go 签名：`func X(ctx context.Context, req *dto.XReq) (*dto.XResp, error)`；`ctx` 为首参；返回 `(result, error)`，不得吞错。
- 错误用 `fmt.Errorf("...: %w", err)` 包装；对外错误经 `errcode` 映射。
- 单一职责、命名即语义；魔法值用常量/枚举；公共函数写用途/参数/返回/错误注释。
- 分层职责不可越界：handler 不写业务与 SQL；service 管事务与规则并暴露 interface；repository 只做数据访问；model 只有表结构、dto 只有契约。
- 前端全量 TypeScript（`<script setup lang="ts">`），禁用隐式 any；类型契约在 `src/types/` 手工维护，**不做自动生成**。
- **安全加固（T18）**：① **限流** `middleware.RateLimit(perMin)`（固定窗口，键为客户端 IP：`X-Forwarded-For` 首个 → `X-Real-IP` → `RemoteAddr`）；超限返回 **HTTP 200 + `code 14` + `Retry-After`**（*必须保持 HTTP 200*：前端 axios 只解包 body，4xx 会走异常分支拿不到结构化消息）。`engine.Use(...)` 在 `/api/health` 之后调用（探活不吃配额），登录/注册再套更严格的一层（`Security.RateLimitPerMin` / `AuthRateLimitPerMin`，0 = 关闭）。② **上传** `handler.UploadImg`：大小上限 → 扩展名白名单 → **文件头嗅探**（`pkg/fileutil.SniffAllowed`，防改名绕过）→ **随机文件名**（32 位 hex，防枚举）；规则集中在 `internal/pkg/fileutil`（含单测）。③ **静态目录**：`/uploads/*path` 走 `handler.ServeUpload`（**替代 `engine.Static`**）——仅白名单扩展名、`Content-Type` 由扩展名固定（绝不 `text/html`）、`nosniff` + `inline`；配置 `UPLOAD_SIGN_KEY` 时校验 `?s=<hmac>`（默认关闭，保证分享页内嵌图片可用）。
- **回收站批量操作（T20）**：接口/文档回收站支持**多选批量还原**与**批量彻底删除**。① 批量还原 `POST /api/api/restore-batch`、`POST /api/doc/restore-batch`（body `{ids:[...]}`）：按条目所属项目校验 **update** 位 → **把「已删除的所属分组（含祖先链）」一并恢复**（`deletedGroupChain`，避免还原后掉进「未分组」丢失目录结构）→ 恢复条目，返回 `{restored, groups}` 并写日志。② 批量彻底删除 `POST /api/api/purge-batch`、`POST /api/doc/purge-batch`：需 **delete** 位，走 `gmPurgeMany`（`Unscoped().Delete`，**物理删除不可恢复**），返回 `{purged}`。③ 前端回收站（`ApiListView`/`DocListView` 的 `?trash=1`）：`.trash-bar`（全选本页 + 已选计数 + 批量还原/彻底删除）+ 勾选列与**「操作」列冻结**（`app.css` 的 `.trash-table .col-check/.col-ops` 用 `position: sticky`），翻页/搜索会清空勾选。④ **分页条数可选**：`components/TablePager.vue` 用 `n-pagination` 自带的 size picker，**只有传入 `pageSizes` 才显示**（默认不传 ⇒ 其余 9 处列表外观与行为不变）；回收站传 `[10,20,40,80,100,200]`（对齐后端 `internal/pkg/page.Normalize` 的 200 上限），改条数后回到第 1 页并清空勾选（`onPageSize` → `reload(1)`）。**注意**：`n-checkbox` 在此版本渲染为 `div[role=checkbox]`（无 `input`），写测试选择器要用 `.n-checkbox`。
- **工程门禁（T11）**：`scripts/check.sh` 一键跑（gofmt 检查 → go build → go vet → go test → vue-tsc → eslint → vite build，`--skip-build` 可跳过最后一步），`.github/workflows/ci.yml` 与其同源（**仓库暂无 git remote，接入后自动生效**）。前端 ESLint 为 `web/eslint.config.js` 扁平配置的**推荐集**（`npm run lint` / `lint:fix`），**刻意不引入 Prettier**：纯格式规则大多显式关闭，只拦真问题；`*.d.ts` 放宽 `no-empty-object-type`/`no-explicit-any`。改代码后请跑 `bash scripts/check.sh`。
- **i18n（中英双语，方案见 `doc/i18n国际化改造.md`）**：前端文案**一律不硬编码中文**，走 `t('模块.语义')`（`const { t } = useI18n()`；`App.vue` 用 `useI18n({ useScope: 'global' })`）。语言包 `web/src/i18n/locales/{zh-CN,en-US}.ts` **key 必须一一对应**，改完跑 `npm run i18n:check`（`web/scripts/i18n-check.mjs`：校验两包 key 一致 + 源码 `t()` 用到的 key 无缺失，有缺失退出码 1）。**非组件代码**（`utils/`、无 setup 的 composable）用 `translate()`（`@/i18n`），组件内仍用 `useI18n()`。**命名空间约定**：通用词（保存/取消/全选本页/已选 N 项/回收站为空，以及中英不同分隔符 `common.listSep`/`common.sentenceSep`）进 `common`；变量表单字段（名称/值/状态/敏感值/空值/变量名规则）进**共享 `varForm`**（公共属性页与变量页共用）；其余按页面/领域分（`nav`/`api`/`doc`/`docDetail`/`field`/`tree`/`variable`/`param`/`detail`/`property`/`env`/`varText`/`varBadge`/`json`/`search`/`notice`/`user`/`project`/`auth`）。迁移后逐文件自查 `grep -n "[一-龥]" <file> | grep -vE ':\s*(//|\*|/\*\*|<!--|/\*)'` 应为空——**例外（有意保留中文）**：失败响应示例的 `msg` 等示例数据、以及 `msg.includes('已被他人修改')` 的乐观锁冲突判定（后端 `msg` 契约仍中文，待后端补错误码后改判）。语言来源：**用户表 `user_info.lang`（登录/刷新经 `/api/user/me` 同步）> localStorage `apidoc.lang` > 浏览器语言 > zh-CN 兜底**；`lang=''`=未选择、不覆盖本地判定。切换入口在 `components/UserMenu.vue`（二级菜单，key `lang:xx-XX`），经 `composables/locale.ts#useLocaleSwitch` 本地生效 + `PUT /user/profile {lang}` 回写用户表（本人可改，白名单外返回 22；白名单 `model.LangZhCN/LangEnUS` 与前端 `LOCALES` 必须同步改）。naive-ui 内置文案（分页、输入框、空态等）由 `App.vue` 的 `n-config-provider` 跟随语言（`zhCN/dateZhCN` ↔ `enUS/dateEnUS`），**新增全局 provider 时要保持这一联动**。**后端 `msg` 仍为中文**（前端按错误码兜底，批次 4 再处理）；迁移 `0009_user_lang` 已就位（`user_info.lang`）。
- **文档导出（T14）**：`utils/export-doc.ts` 把接口/项目导出为 **Markdown** 或**自包含单文件 HTML**（内嵌样式 + 侧边目录，窄屏自适应），Markdown 正文用 `markdown-it`（`html:false` 防注入）渲染；入参为宽松结构体 `ApiExportInput`/`ProjectExportInput`（接口详情页可直接用「编辑态表单」构造，导出即含未保存改动）。入口：接口详情 `.req-ops` 的「导出」下拉（单接口）、项目概况「导出文档」下拉（OpenAPI / Postman / Markdown / HTML 四项）。**不新增后端接口**。
- **顶栏全局搜索**：`components/GlobalSearch.vue`（`AppLayout` 顶栏中间）——去抖 300ms 调 `api/content.ts` 的 `searchAll`，结果分组面板（接口/文档/项目）支持上下键与回车；命中项跳 `projectApis?api=` / `projectDocs?doc=` / 项目接口页。**公共属性页（`views/PropertyView.vue`）为双页签**：「标签字典」与「全局变量」（仅管理员可见，复用 `api/variable.ts` 的 `scope=3` 接口，敏感值编辑时按 `with_value=1` 单独取明文回显）。详情页「代码」下拉用 `NDropdown` + 预览弹窗（含「复制 Mock 地址」），代码块类名是 `.snippet`（**勿用 `.code-block`**：`JsonBlock.vue` 已占用该类名）。
- **移动端适配（≤768px，另在 ≤900px 收起工作台两栏）**：窄屏规则集中在 `styles/app.css` 的 `@media (max-width: 768px)`（页面留白收紧、标题缩小、`.toolbar/.row-actions/.filter` 允许换行、`.table-scroll` 横向滚动、**`.n-modal` 必须用 `width/max-width: 94vw !important` 覆盖 naive-ui 的内联宽度**、`.n-drawer` 同理）与各组件自己的媒体块（`AppLayout` 顶栏换行 + 页签横滑、`GlobalSearch` 搜索独占一行且结果面板占满屏宽、`ApiDetailView` URL 栏换行且**隐藏悬浮大纲 `.req-ol`**、`VariableView` 筛选栏占满宽度、`ShareView` 两栏改上下）。`app.css` 的 ≤900px 块把 `.split` 改为 column 并让 `.split-tree` **`width: auto`**（否则列布局下树只有 272px 宽）。
- 前端布局：`AppLayout` = 顶栏（品牌 `RepoMark` 仓库图标 + 文案 `API-DOC-GO`，与 `document.title`/`index.html` 保持一致；右侧为**通知铃铛** + **头像悬停二级菜单**（修改资料 / 修改密码 / 退出登录，点选后居中弹窗操作，`components/UserMenu.vue`）+ **项目切换 `n-select`（紧跟品牌标题、加粗 + 外框，`size="medium"` 34px 更显眼，箭头贴近项目名：`--n-padding-single-right: 18px`、`.n-base-suffix { right: 4px }`）**）+ **横向 tab 栏（三栏：左 `.tabbar-left` 圆形**「主页」按钮** `.back-btn`（`HomeOutline` 图标、`title/aria-label="主页"`、`!isHome` 才显示；点击**固定回项目列表** `router.push({name:'projects'})`——**不做浏览器级返回**，历史栈多深都回首页）/ 中间居中页签 / 右 `.tabbar-right`）**；**已移除面包屑路径行**（原 `.crumb-bar` 与 `crumbs` 一起删除），页面操作改由 `.page-actions#crumb-actions` 承接（**`<teleport defer>` 注入导入/导出/搜索的目标，位于 tab 栏右侧、回收站 tab 之前**）。tab/回收站逻辑集中在 **`composables/projectNav.ts`**（导出 `tabs/activeTab/trashActive/onTab/switchProject`）：项目内 tab = 项目概况/API接口/项目文档/字段映射/**备忘清单**（原 Bug 管理，支持置顶且列表置顶优先）（**无数据字典页**：原「链接外部库」功能已整体移除（`/api/database*` 路由、`datasource`/`crypto` 包、`DatabaseConn` 模型、`SECRET_ENCRYPT_KEY` 一并删除，迁移 `0005_drop_database`），本地字段维护仍由「字段映射」页承担；**无操作日志页**：日志合并进项目概况底部，`projectLogs` 路由与 `OpLogView.vue` 已删除），全局 tab = 项目列表/用户管理/公共属性；**回收站 tab 右对齐且按当前分区**（`apis`/`docs` → `?trash=1`，默认 API）。**默认进入项目即 API接口**（`ProjectListView` 链接与 `switchProject` 指向 `projectApis`）。**接口/文档为工作台布局**：`ApiListView`/`DocListView` = 左 `SectionTree.vue`（目录节点 + 其下**接口/文档叶子**，顶部**树上搜索**过滤叶子，最多 7 级；数据源 `stores/project.ts` 分组 + `listApis/listDocs` 全量叶子，`utils/group-tree.ts` 按 `p_id` 组树）+ 右侧内容（`?api=<id>`/`?doc=<id>` 内嵌 `ApiDetailView`（props `apiId`，支持 `?edit=1`）/`DocDetailView`（props `embedded`）、`?new=1&group=<id>` 新建接口全屏（按 `?group` 预选所属目录；**本页不设独立「创建接口」按钮，新建一律走树上目录「＋」菜单**，导入/导出也移至项目概况页）、`?trash=1` 回收站表格、**无选中时 `EmptyHint` 空态（大号浅色图标 + 引导提示，接口页 `SendOutline`/文档页 `DocumentTextOutline`）**）。原 `apiDetail`/`docDetail` 独立路由与 `GroupTree.vue` 已删除。项目内**一屏一路由**（`views/project/*View.vue`）。**接口页/文档页为 Postman 风格全出血两栏**：`AppLayout` 对 `projectApis`/`projectDocs` 取消 `.page-body` 内边距（`.page-body--flush`）并给 `.content-inner` 加 `--flush`（`height:100%`，让外壳不滚、工作台各栏自己滚）。`SectionTree` 为扁平集合面板：顶部计数（**彩色圆圈徽标**）+ **全部展开/全部收起**图标 + **新建目录**图标（+ 接口页的「收起左栏」，收起后 `.split-tree--collapsed` 32px 窄条 + `.tree-rail` 展开）；**目录与其下叶子都可展开/收起**（含只含接口的目录，`hasChildren` 仅指子目录，故可展开性另由 `groupExpandable`＝有子目录或有叶子判定；`expanded` 只在首次加载全展开、之后仅补齐新目录，避免刷新覆盖用户手动的收起）；目录行 = 折叠箭头 + 文件夹图标 + **悬停操作：`n-dropdown`「新建子目录/新建接口」+ 重命名 + 删除**；叶子行 = **方法描边徽标**（`MethodTag`，Postman 配色，浅底深字 + 同色描边）+ 名称 + **悬停删除**（删除当前选中项后回到空态）。右侧 `ApiDetailView` 为请求编辑器（**名称栏 = 接口名 + `cURL`，无关闭按钮**；查看态在名称后依次跟「请求协议」「所属分组」小胶囊，编辑态同一位置是所属分组下拉 + **URL 栏**（方法下拉/方法徽标 + URL + 复制 URL + 编辑/保存主按钮）+ **各段垂直排列**：请求头部 / 请求参数 / 返回参数 / 响应样例（成功 + 通用失败）/ 基本信息，段间用 `.req-sec + .req-sec` 上分隔线；末尾是独立的**「文档」一节**（该接口的 Markdown 说明，放最底部）：**编辑用 `MdEditor`**（工具条 + 分栏实时预览，图床走 `composables/md-upload.ts`，定高 340px）、**查看用 `MdPreview`** 渲染（Bruno `docs` 即为 Markdown）；「基本信息」查看态只在「开发语言」有值时出现，编辑态不再有该节；**返回类型以 `ApiParamTable` 的 `title-extra` 徽标展示在「返回参数」标题旁**；详情区还有两个悬浮件——**大纲 `.req-ol`**（默认收起为「一节一个小格子」的 2 列方格、悬停展开为章节列表、点击 `scrollToSection` 平滑跳段，滚动时按 `data-sec` 做高亮 scroll-spy）与**返回顶部 `.req-top`**（`.req-body` 滚动超过 240px 才出现））；接口页/文档页的整块内容（新建接口、回收站）用 `.pane-card` 留白，详情类内容用 `.pane`（也需给 `.n-spin-content` 撑满）。**项目概况页承载导入/导出**：头部按钮「导入接口 / 导出 OpenAPI」，导入弹窗支持数据来源（OpenAPI 文件 / **Bruno 集合目录**，`n-upload directory` 读目录内 yml，`show-file-list=false` 改用自带 `.flist`（**定高 132px + 内部滚动**）；`update:file-list` 会分批触发，故读取**去抖 300ms 合并为一次**并只弹一次「共读取 N 个文件」，避免刷屏）与导入策略（`append` 只新增并按方法+路径去重 / `overwrite` 清空项目接口与接口目录后整体导入），结果返回 `{imported, skipped, failed, failures}`，导入完成弹出**「导入结果」弹窗**（成功/跳过/失败三行 + 失败明细最多 10 条，`确认` 关闭，覆盖策略附提示）；`api.source` 记录来源（`openapi-import`/`bruno-import`，空为手工）。**项目列表为卡片栅格**（`.cards/.pcard`，**整卡可点击进项目**，操作按钮 `@click.stop`）。**登录/注册卡片带标题与图标**（`.login-head/.login-icon`）。**改资料/改密/退出登录都走居中弹窗**：`components/ProfileModal.vue`、`components/PasswordModal.vue` 均以 `v-model:show` 复用（顶栏 `UserMenu` 与个人中心共用），退出清理由 `composables/logout.ts` 统一（`UserAccountPanel.vue` 已删除）。复用 `components/{SectionTree,ApiParamTable,ApiFieldsEditor,EmptyHint,MethodTag,JsonBlock,TablePager,RepoMark,UserMenu,NotificationBell,ProfileModal,PasswordModal}.vue` 与 `styles/app.css` 的页面类（`.page/.page-hd/.toolbar/.row-actions/.table-scroll/.split/.split-tree/.split-tree--collapsed/.tree-rail/.split-main/.pane/.pane-card`）；**接口参数/请求头/返回参数表一律用 `ApiParamTable`**（`kind=request|response|header`，查看/编辑同版式；段标题的计数为**彩色圆圈**（各形态一色）；类型列是 **JSON 类型下拉**（`filterable` + `tag`，可选可自填）；对象/数组行可「添加子字段」，嵌套用 `data.list[].id` 命名表达），⚠️ **请求头在组件间一律以 `ApiParam[]` 承载**（`ApiFieldsEditor`/`ApiDetailView`/`ApiListView` 的 `headerRows`），只在**载入与保存**时用 `utils/api-params.ts` 的 `headersToParams`/`paramsToHeaders` 互转——**勿用 `computed` 包一层再让表格改数组项**：转出的对象是非响应式副本，输入会被原生 v-model 的 beforeUpdate 回滚且保存丢失；**列表一律用 `TablePager` 常驻分页**（数据不足一页也显示「共 N 条」，勿写 `v-if="total > pageSize"`；后端不分页的接口在前端切片分页）；**带分页的列表卡片用 `.table-card` 纵向 flex 撑满**，`TablePager` 靠 `margin-top:auto` 贴卡片底；**新建长表单/回收站覆盖整个内容区（隐藏左树）**；**内容区背景须挂在滚动容器 `.content`（flex 撑满）上**；行内跳转用 `router-link`（不用 span 绑 click）；naive-ui 容器子节点必须配套子组件（`n-grid` → `n-grid-item`，`n-tabs` → `n-tab-pane`），否则被静默丢弃；⚠️ naive-ui `NCard` 内容层**实际类名是 `n-card-content`（单连字符）**，写 CSS 命中它时一并兼容 `n-card__content` 以防版本差异；⚠️ `gofmt -w` 只跑 `cmd internal`（`data/_modcache` 为只读 modcache，勿全仓格式化）。
- 后端查询参数命名以 `doc/api_docs/*.yml` 为准（如 `project_id`/`bug_id`）；handler 用 `idOrFallback(c, name)` 解析，勿只读 `id`。
- **并发编辑保护（乐观锁）**：`Api`/`Doc`/`Variable` 的更新 DTO 带可选 `expected_update_time`（客户端载入时的 `update_time`，RFC3339）；传入时 `service.ensureNotStale` 与库中 `update_time` 比对，不一致返回 `CodeFailed` + 「该X已被他人修改，请重新载入最新内容后再保存」；**不传则完全保持历史行为**（整体覆盖）——新增可编辑实体时沿用该约定（前端保存/置顶都要回传，冲突后引导重新载入；已接入：接口详情、文档详情与置顶、变量管理页、公共属性全局变量页）。
- **分享链接（免登录只读）**：`share_link` 表（`token` 32 位随机十六进制、局部唯一、可设 `expire_time`）。受保护接口 `POST/GET/DELETE /api/share`（需项目 `update` 位）负责创建/列表/失效；**公开接口 `GET /api/share/:token` 免登录**（`service.SharePayload` 校验令牌与有效期，返回项目 + 分组 + 接口 + 文档）。前端路由 `/share/:token`（`views/ShareView.vue`）= 只读文档站视图（左目录树 + 右接口/文档详情，复用 `ApiParamTable`/`JsonBlock`/`MdPreview`，**无任何编辑入口**）；管理入口在项目概况的 `components/ShareLinksPanel.vue`（生成/复制/失效）。
- **Mock 服务**：`ANY /mock/:project_id/*path`（**免登录**，注册在公共路由）按「方法 + 路径」匹配项目接口并返回 `http_return_sample`（其中 `{{变量}}` 按**通用变量** env_id=0 渲染）；完全未命中返回 **HTTP 404** JSON，方法不匹配但路径命中的兜底返回样例。dev 下 Vite 已代理 `/mock`（`vite.config.ts`）；接口详情「代码」下拉提供「复制 Mock 地址」。
- **Postman 互导**：`GET /api/api/export-postman?project_id=&group_id=`（Collection v2.1：分组 → 文件夹，接口 → request + response 样例）；`POST /api/api/import-postman?project_id=&strategy=`（body 为集合 JSON：文件夹 → 分组、URL 查询串与 raw JSON 体展平为请求参数、请求头写入 `http_request_header`、去重口径与 OpenAPI/Bruno 一致、来源标记 `postman-import`）。前端入口：项目概况「导出 Postman」按钮 + 导入弹窗（来源三选一：OpenAPI / Bruno / Postman；**不支持也不打算做 OpenAPI YAML 文本粘贴导入**）。
- **文档图标**：`doc.icon`（emoji 或上传图片 URL，空 = 默认图标）随 `doc` 接口读写；`utils/icon.ts` 的 `isImageIcon` 决定用 `<img>` 还是文本渲染，展示在左树叶子（`.gn-icon`/`.gn-icon-img`）、文档详情标题与分享页目录树；编辑态在文档详情表单（填 emoji 或「上传图片」走 `/file/upload`）。**注意整体覆盖式保存（如置顶 `toggleTop`）必须回传 `icon`，否则会被清空**。
- **请求代码与 cURL**：`service/code.go` 统一生成 cURL / fetch / axios / go / python 片段（`GET /api/api/code?api_id=&env_id=&lang=&resolve=`，cURL 快捷入口仍是 `GET /api/api/curl?id=&env_id=&resolve=`，两者同源）。`resolve=1` 时用 `resolveTable`（原文，含敏感值）+ `renderVars` 服务端渲染，未定义占位符原样保留、空值替换为空串；**`resolve=0`（默认）必须与历史输出逐字节一致**（单测锁定）。敏感值参与渲染时写 `recordOp(action=export)` 审计：**只记接口名与变量名，绝不记值**。
- **全局搜索**：`GET /api/search?keyword=&limit=`（`repository/search.go` + `service/search.go`），覆盖接口名/URL/说明、**文档正文**、项目名，名称命中优先、每组按 `limit` 截断（默认 10、上限 50）；**可见性由 `searchableProjectIDs` 统一决定**（管理员全量、其余仅 `user_project` 参与的项目），新增搜索维度必须走同一入口以免越权。
- 接口文档 yml 与后端 DTO 字段/错误码/分页结构保持一致；接口变更同步更新 yml。变量/环境契约在 `api_docs/variable/`（`var_*.yml` 变量、`ve_*.yml` 环境），**表格行缩进 2 空格**（`docs: |-` 是块标量，顶格行会破坏 YAML——既有 69 个历史 yml 存在该问题，未批量修）。

## 关键不变式
- 业务错误码沿用：`200 成功 / 22 参数错误 / 14 失败 / 34 未登录或过期 / 403 无权限`；`34` 前端会跳登录，勿改。
- 软删除统一 `is_deleted`（整型 0/1），列表查询默认过滤已删除。
- **唯一约束必须排除软删除行**：带唯一性的字段用局部唯一索引（如 `project.title` → `uniqueIndex:uk_project_title,where:is_deleted = 0`，`database.Migrate` 会清理历史全局索引 `idx_project_title`），否则删除后同名记录无法重建（列表看不到却报重名）。
- **变量与环境（三级变量）**：`variable`（`scope` 1 请求级（绑 `api_id`）/ 2 项目级（`env_id=0` 通用、`>0` 绑环境）/ 3 全局（应用级，`project_id=0`，**仅管理员可写**，其余写操作按项目 `create/update/delete` 位））+ `variable_env`（项目级环境，同项目默认环境唯一；首个环境自动为默认）。**解析优先级：请求级 > 项目级(当前环境) > 项目级(通用) > 全局**；停用（`enabled=0`）与绑定其它环境的项目变量不参与解析，未命中即「未定义」（前端黄色告警，空值另作中性提示）。同层同环境唯一由局部唯一索引 `uk_variable_key(scope,project_id,api_id,env_id,name) WHERE is_deleted=0` 兜底；变量名规则 `^[A-Za-z_][A-Za-z0-9_]{0,63}$`（**禁 `$` 前缀**，为将来兼容 `{{$randomXxx}}` 保留）。**敏感值 `is_secret=1` 明文入库（v1 不加密）：`is_secret` 只是展示脱敏标记、不是安全边界**——列表/`resolve`/`summary` 一律只回掩码 `••••••` + `has_value`（**唯一出口是 service 的 `maskVariable`**），`with_value=1` 时对具备写权限者回明文并写操作日志。`GET /api/variable/resolve`（单接口解析表）与 `GET /api/variable/summary`（列表徽标批量，单次上限 200 个接口，`api_ids` 逗号分隔）**共用同一套解析器**，两个界面口径必须一致。**删除环境：默认环境（项目仍有其它环境时）与仍被变量引用的环境一律拒绝**并返回明细文案（预检 `GET /api/variable-env/references?id=`，前端据此禁用删除并列出引用变量名；唯一的默认环境允许删除，项目回到无环境状态）。前端入口：项目内「变量」tab（`views/project/VariableView.vue` = 环境胶囊条（颜色点 + 默认标记 + 变量计数 + 悬停操作）+ 变量表 + 变量/环境弹窗，`API_TABS` 与路由 `projects/:projectId/variables`，`CodeSlashOutline` 图标）。**全局变量规划在「公共属性」页维护（后续阶段）**，变量页内对其只读。**变量的展示层**：`stores/variable.ts`（当前环境按项目记 `localStorage`（`apidoc.env.<projectId>`，被删环境自动回落默认）+ 解析表/徽标缓存，**缓存键一律带 `projectId:envId`**，切环境自然换键；变量/环境变更后调 `invalidate(projectId)`——由变量页在每次 reloadAll 时调用，同时 `loadEnvs(projectId, true)` 刷新共享环境列表）、`utils/vartext.ts`（`splitVarSegments` 切片 + `sourceLabel`，正则与后端一致）、`components/VarText.vue`（`{{var}}` 就地渲染：**有值=强调绿、敏感值=紫、空值=中性灰、未定义=黄色告警底**，悬停 tooltip 给「变量名 · 来源 · 取值」；未传 projectId/apiId 时退化为纯文本）、`components/EnvPicker.vue`（名称栏右侧环境下拉，选项带颜色点与默认标记）、`components/VarBadge.vue`（请求变量数 + 未定义数小圆标，悬停列变量名清单）。**接入点**：`ApiDetailView` 的 URL 栏（查看态用 `.url-static` + VarText，编辑态仍是输入框）与三张 `ApiParamTable`（新增 `projectId`/`apiId` props，查看态「值/示例」列接 VarText）。**左树不展示变量统计**（2026-09-26 按需求移除：`SectionTree` 的叶子徽标、`loadSummaries()` 与 `watch(currentEnv)` 均已删）。`components/VarBadge.vue`、store 的 `summaryOf`/`summaryFor` 与后端 `GET /api/variable/summary`（含单测与 `api_docs/variable/var_summary.yml`）**保留但当前无 UI 调用**——将来做「API 列表」表格或其它入口时可直接复用，口径与 `resolve` 一致（同一解析器）。
- 数据库方言无关：业务 SQL 不写 sqlite/pg 专属语法；差异只在 `config`/`database`/`migrations`。
- 不做旧项目历史数据迁移；新库经 AutoMigrate/迁移全新建表 + 必要种子（如超管）。

## 当前进度
见 `doc/项目进度.md`。**P0–P11 全部完成**（后端全功能 + 前端全页面，含回收站/OpenAPI 导入导出/cURL/introspect/日志清理/属性管理/图床上传），单测 4 包、`api_docs/` 逐接口 yml 与 `migrations/` 生产 SQL 已备。前端已按 `project-overview/` 旧版截图**完成整站重设计**（侧导航 + 面包屑 + 表格化列表 + 项目内一页一路由）。剩余可选：OpenAPI YAML 文本导入、真实邮件/短信通道、golang-jwt/argon2 升级、CI 门禁。

