# 服务端 API 页对齐客户端实施清单

> 目标：把 Web「文档」工作台里的 API 详情页做成**和客户端 `RequestEditor` 同构的纵向长页** ——
> 每个段落对应客户端的一个 tab，段内的字段、列序、样式、行操作逐项对齐。
> 基线：2026-11-05，`web/src/views/project/ApiDetailView.vue`（纵向长页，10 个 section，右侧悬浮大纲）。
> 对照：`api-doc-go-client/frontend/src/components/RequestEditor.vue`（9 个分段）+ `KeyValueTable.vue` /
> `BodyFieldsTable.vue` / `ResponseFields.vue` / `RequestDocsPane.vue`。

## 一、本轮确认（定稿，后续方案不得回退）

1. **不移植运行时能力。** 客户端的分段只是「编辑形态」，真正的价值在本地 HTTP 引擎（发送、cookie jar、
   环境变量解析、脚本/断言执行、快捷键、右侧响应面板）。Web 没有也不该有这些，**只搬编辑形态**：
   不新增发送链路、不加右侧常驻响应栏、不做脚本执行/变量插入器/断言校验。
   现有「调试」弹窗（`TryItPanel`，服务端代发）原样保留，不属于本轮改造范围。
2. **不做 tab 化。** 保留现在的纵向长页与悬浮大纲；把客户端**横向 tab 条竖排成纵向段**，
   段的顺序、段内字段（列）、样式（grid 列宽/sticky 表头/字号）、行操作（启用勾选、删除、加行）与客户端逐项对齐。

结论先行：**能对齐的是「一张表格长什么样、哪几列、怎么增删行」；不能对齐的是「点了发送之后」。**

## 二、对齐边界（先划线，避免越做越重）

| 客户端能力 | Web 处理 | 理由 |
|---|---|---|
| 9 个横向 tab | **竖排为 8 个纵向段**（顺序一致，见 §三） | 长页 + 大纲已有，不需要引入 tab 状态与深链接 |
| `enabled` 启用勾选（参数/请求头/表单体） | **对齐**：新增列，跨三仓落库（§六） | 这是行级数据，不是运行时能力 |
| 请求体字段表 / 响应字段表 5 列 | **对齐**：列序、列宽、sticky 表头、行尾删除、底部加行 | 纯编辑形态 |
| 请求体类型切换 + 格式化 / 复制 / JSON 语法提示 | **对齐** | 纯前端文本处理 |
| 「生成数据表格」（从 JSON 派生字段表） | **可选**（P2 标记可选，见 §5.2） | 纯函数派生，但要与客户端 `deriveBodyFields` 语义一致 |
| 认证表单（type/字段联动） | **对齐** | 纯编辑形态 |
| 发送 / 右侧响应面板 / 状态码 / 响应头快照 | **不做** | 无本地引擎；`respDoc.status/headers` 只回传不编辑 |
| 变量 / 脚本 / 断言 | **只读展示**，不给编辑框 | 编辑了也无法执行，纯增维护面 |
| 环境解析 `{{var}}` 高亮 | **保留现状**（只读高亮） | 已有 `VarText`，不扩插入器 |
| 快捷键、页内查找、批量编辑 | **不做**（批量编辑待评估，非本期） | 与「不移植运行时」同源，避免交互面膨胀 |

## 三、目标形态：8 个纵向段

段序沿用客户端 tab 顺序，仅把「请求头」提到「请求参数」之前（Web 阅读习惯：先头后参）：
请求头 → 请求参数 → 请求体 → 认证 → 响应字段 → 响应示例 → 变量/脚本/断言 → 文档。
Web 侧把「变量/脚本/断言」合成一个只读段，「响应示例」作为 Web 专有段紧随响应字段。

| 段序 | `data-sec` | 段标题 | 数据源列 | 客户端对应 | 编辑能力 |
|---|---|---|---|---|---|
| 1 | `headers` | 请求头 | `http_request_header` | `headers`（`KeyValueTable`） | 全编辑 + 启用 |
| 2 | `params` | 请求参数 | `http_request_params` | `params`（`KeyValueTable`） | 全编辑 + 启用 |
| 3 | `body` | 请求体 | `http_request_body_type` / `_body` / `_form` / `_fields` / `_descs` | `body` | 全编辑 |
| 4 | `auth` | 认证 | `http_auth` | `auth` | 全编辑 |
| 5 | `result` | 响应字段 | `http_return_params` + `http_return_content_type` | `resp`（`ResponseFields`） | 全编辑（无启用列） |
| 6 | `sample` | 响应示例 | `http_return_sample` + `api_return_sample` | 无（Web 专有） | 现有能力不变 |
| 7 | `runtime` | 变量 / 脚本 / 断言 | `script_vars` / `http_script` / `script_asserts` | `vars` / `script` / `tests` | **只读** |
| 8 | `doc` | 文档 | `description` | `docs`（`RequestDocsPane`） | 现有 md-editor |

实现要点：

1. `sections` 计算属性与模板中的 `data-sec` 一一对应，顺序即上表；大纲、滚动高亮、`scrollToSection` 跟着改。
2. 「接口设置」仍在右上齿轮弹窗（`detail.settings`），不占纵向段；状态/版本/分组/概述仍在头部。
3. 查看态空段不占位（现状保留）；**编辑态全部显示**，便于从零补齐。
4. 不做 tab 圆点：段标题后改用**计数徽标**（表格类段显示行数，请求体/响应字段显示类型徽标），
   与客户端 `body.fields` / `ResponseFields` 标题旁的 `<span class="num">` 同观感。

## 四、通用表格与行操作规范（一套，四张表复用）

客户端三种表其实是两种形态，Web 统一按下面这套实现（不再各段自己写一套）：

| 规范项 | 要求（对齐客户端） |
|---|---|
| 布局 | `div` + `grid`，不用 `<table>`；`gap: 8px`，行 `padding: 5px 8px`，字号 `12px` |
| 表头 | `position: sticky; top: 0`，背景 `--app-surface-2`，字号 `11.5px`，颜色 `--app-muted` |
| 列宽 | 见下表，全部用 `minmax(0, N fr)` + 固定 px 操作列，**保证不随内容跳动** |
| 行 hover | `background: var(--app-hover-soft)` |
| 删除 | **常显**的行尾图标按钮（悬停变红）——旧稿写的「hover 才显示删除」与源码不符，纠正：客户端删除按钮常显，hover 才出现的是 **列宽拖拽手柄**（`.rs`） |
| 列宽拖拽 | 仅键值表（请求参数 / 请求头 / form-multipart）：拖 name/value/desc 右缘改 `fr` 权重，`MIN_FR = 0.3` |
| 加行 | 表格底部 `+ 添加一行`（键值表是文字链接，字段表是 `size=tiny` 按钮） |
| 自动补空行 | 编辑态末行一旦有输入就**就地补一行空行**；补的空行不产生脏标记（对齐 `ensureTrailingBlank`） |
| 空态 | 查看态无数据：表格边框内一行 `12px` 灰字（不是虚线大框），点击切到编辑态 |
| sticky 前提 | 段与其祖先不能有 `overflow: hidden/auto`，否则表头粘不住（滚动容器是 `.req-body`） |

各表列宽（严格照抄客户端）：

| 表 | grid-template-columns |
|---|---|
| 请求参数（启用 + 文档 5 列 + 加子字段/删除） | `24px minmax(0,1.3fr) 108px 40px minmax(0,1fr) minmax(0,1.4fr) 56px` |
| 请求头（启用 + 3 列） | `24px minmax(0,1.1fr) minmax(0,1.4fr) minmax(0,1.4fr) 24px` |
| 请求体字段表 | `minmax(0,1.3fr) 108px 40px minmax(0,1fr) minmax(0,1.4fr) 32px` |
| 响应字段表（含加子字段/删除） | `minmax(0,1.3fr) 108px 44px minmax(0,1fr) minmax(0,1.4fr) 56px` |
| form/multipart（含 type / file 选择） | `24px 1.1fr 1.4fr 88px 1.4fr 24px` |

> 请求参数比客户端多 `type` + `required` 两列，这是**有意的偏离**：客户端 query 表服务于「发请求」，
> Web 参数表服务于「写文档」，服务端行契约（`apiParamRow`）本身就有 type/required，删掉会导致文档与导入导出丢失语义。
> 其余列序、宽度、操作按客户端排（启用 → 名称 → 类型 → 必填 → 示例值 → 说明 → 删除）。

## 五、逐段规格

### 5.1 请求参数（`params`）

- 行结构：启用 / 字段名 / 类型（select，filterable+tag，可自定义）/ 必填（勾选）/ 示例值 / 说明 / 删除。
- 保留现有「JSON 转表格」「复制 JSON」「对象行加子字段（`data.list[].id`）」能力。
- 新增启用列语义见 §六；勾掉的行在导出/调试/文档生成中跳过，本页仍显示。

### 5.2 请求体（`body`）

按客户端顺序竖排，**原来客户端的「原文 / 表格」子页签改成上下两块**，不再做子 tab：

1. 类型行：`select`（none / json / text / form / multipart，宽 `130px`）+ 右侧小按钮
   （JSON/文本：`格式化`、`复制`；JSON 且语法错：红字「JSON 语法有误」且**不阻断保存**，与客户端一致）。
2. `json` / `text`：正文编辑器（占满宽度的 textarea，`12.5px` 等宽字体）。
3. `form` / `multipart`：键值表（`showType` 时多一列 text/file，`file` 行带「选择文件」按钮）；
   表格下方给一段**只读请求数据示例**（`form` 拼 `a=1&b=2`，`multipart` 拼 boundary 分段示意），
   对齐客户端 `formExample`，仅展示、不参与保存。
4. 「请求体字段」块（JSON 体时）：标题 + 行数徽标 + 说明 + 5 列表格（§四）；
   右上角 `生成数据表格`（可选实现，见下）；底部 `+ 新增行`。
5. 字段表真空态：边框内一行灰字（对齐 `body.empty`），编辑态直接显示可编辑空行。

`生成数据表格`（可选）：复用 Web 已有 `utils/json-params.ts` 的展平能力产出 5 列，
语义必须等于客户端 `lib/bodyFields.deriveBodyFields`（对象按键名字典序、数组取首元素、`a.b` / `a[].c` 路径）；
已有行时弹「新增 / 覆盖」二选一。**如果不做**，就只保留手工加行与客户端同步过来的表格，不出现半截派生。

### 5.3 请求头（`headers`）

- 行结构：启用 / 头名 / 值 / 说明 / 删除；`fields ←→ ApiHeader[]` 互转时**带上 `enabled`**
  （`utils/api-params.ts` 两个方向都要改，默认 `true`）。
- 空态与加行按 §四；不再复用参数表组件的 `withRequired` 分支去占位。

### 5.4 认证（`auth`）

- 对齐客户端尺寸：`max-width: 560px`，标签列固定 `90px`，type 下拉 `160px`，密码类输入 `show-password-on="click"`。
- 类型联动：`none` / `inherit` 不显示后续字段；`basic` 用户名+密码；`bearer` token；`apikey` key+value+in(header/query)。
- 查看态保持现有 `dl.kv`，但密码/token 一律 `••••••`（现状已如此，勿回退）。
- 序列化仍保留未知键（`authBase` 透传），别把客户端写入的其它字段洗掉。

### 5.5 响应字段（`resp`）

- 5 列全可编辑：字段名 / 类型 / 必填 / 示例值 / 说明 + 行尾删除 + 底部加行（对齐 `ResponseFields`），
  **没有启用列**（客户端响应字段本来就没有 `enabled`，加了就得新造契约）。
- 新建行默认 `required = true`（客户端与 Web 现有口径一致）。
- `响应内容类型`：从段底部的普通行改为**段标题行右侧**的内联 select（对齐客户端 `resp.contentType` 位置），
  选项仍为 `application/json / application/xml / text/plain / text/html`，可自由填写。
- 「从当前响应生成」不做（无运行时响应）；「从响应示例生成字段」可保留（现有 JSON 导入弹窗已覆盖）。

### 5.6 响应示例（`sample`，Web 专有）

- 保持现状：成功示例 + 失败示例（固定 `{code:22}`）+ 多示例（主示例走 `http_return_sample` 列，多示例走 `api_return_sample` 表）。
- **唯一保存入口仍是顶部「保存」**：编辑态保存时把示例正文一并提交（现有逻辑，勿改）。
- 多示例的独立「保存/删除」按钮只在**查看态**出现；编辑态隐藏，避免两个保存入口。

### 5.7 变量 / 脚本 / 断言（`runtime`，只读）

- 一段只读展示（`kv` 表），标题「变量 / 脚本 / 断言」，有内容才出现。
- 不提供编辑框、不提供插入器、不提供执行按钮；文案明确写「在桌面客户端编辑与运行」。

### 5.8 文档（`docs`）

- 编辑态 `MdEditor`（保持 md-editor-v3 + 本地图片上传），查看态 `MdPreview`。
- 工具条按钮对齐客户端 `.gin` 观感（`--app-accent-tint` 底 + `--app-accent-dark` 字，`12px`，`padding 4px 10px`）。
- 不引入客户端的「生成接口文档」对话框（Web 已有导出 Markdown/HTML，功能重叠）。

## 六、数据契约：`enabled` 必须跨三仓一起改

现状（已核实）：服务端 `http_request_params` / `http_request_header` 的行结构里**没有 `enabled`**，
客户端推送时会把它丢掉（`internal/collection/syncrows.go` 的 `syncParamRow` / `syncHeaderRow`），
拉取时又硬编码 `Enabled: true`。所以**只在 Web 加勾选框等于假装有效**——保存一次就被抹掉。

| 仓 | 文件 | 改动 |
|---|---|---|
| Web | `web/src/types/entities.ts` | `ApiParam` / `ApiHeader` 加 `enabled?: boolean` |
| Web | `web/src/utils/api-params.ts` | 两个方向都映射 `enabled`，缺省 `true` |
| Web | `web/src/components/ApiParamTable.vue` | 参数/请求头加启用列（响应不加） |
| Web | `web/src/views/project/ApiDetailView.vue` | 序列化：`false` 才写 `"enabled": false`；`true`/缺省不写（老数据字节稳定）；读写两侧都按「缺省 true」 |
| 服务端 | `server/internal/service/bruno.go` | `apiParamRow` / `apiHeaderRow` 加 `Enabled *bool`（json tag 带 `omitempty`） |
| 服务端 | `server/internal/service/{extra,bundle_import}.go` | 构造行时不动（`nil` = 启用），无需改逻辑，只确认不被重新序列化洗掉 |
| 客户端 | `internal/collection/syncrows.go` | `syncParamRow` / `syncHeaderRow` 加 `Enabled *bool`；`marshal*` 写出；`apply*` 读入（`nil → true`） |
| 客户端 | `internal/collection/*_test.go` | 往返用例：`enabled:false` 推送→拉取仍是 `false`；老载荷（无该键）→ 默认 `true` |

规则：

- **反序列化缺省 = 启用**（`enabled` 缺键、`null` 都按 `true`）；只有显式 `false` 才停用。
- **序列化只在 `false` 时写键**，避免全量行多出无意义的 `"enabled": true`。
- 行级「空判定」不变：只有 `enabled:false` 的行仍算有内容（不能被清空逻辑当成空行删掉）。
- **发布顺序**：服务端 + 客户端先发（能读 `enabled`），Web 再上勾选框。反序会导致客户端下一次推送把勾选状态全部重置为启用。
- 共享库 `api-doc-go-share` 无需改动（`collection.KV` / `BodyField` 已具备 `Enabled`/`Required`）；跨仓联调用 `bash ../api-doc-go-share/scripts/gowork.sh`。

## 七、保存与校验

1. **唯一入口**：右上角「保存」。切段、滚动、折叠都不产生保存请求；查看态切编辑态不自动保存。
2. **失败定位**：保存前校验失败（示例 JSON 非法、请求体 JSON 非法等）→ `message.error` 指明段名 +
   `scrollToSection(key)` 滚到出错段（避免「报错信息在另一端，用户看不到」）。
3. **示例合并保存**：编辑态的响应示例正文随主保存一起提交（现状），失败不回滚已成功的字段保存
   ——现有实现先 `updateApi` 再存示例，保持；若示例保存失败要明确提示「字段已保存、示例未保存」。
4. **乐观锁**：`expected_update_time` 逻辑保留，冲突弹窗保留。
5. **富字段原样回传**：`ext` 里所有列都必须回传（含本轮新增的 `enabled` 所在 JSON），整条覆盖式更新，缺省即清空。

## 八、实施阶段与验收

### P0 段落骨架（0.5–1 天）

- 段序改为 §三的 8 段；`sections`、大纲、滚动高亮、`scrollToSection` 同步。
- 段标题统一 `12px/600`，段间分隔线保持现状；`runtime` 段改为只读。
- 验收：大纲顺序 = 8 段顺序；`?api=69` 直接落在请求参数段；切段不产生任何请求（Network 面板零保存请求）。

### P1 表格重写（1.5–2 天）

- 抽出通用表格（`ParamTable`/`KeyValueTable` 二选一或合并成 `RowGrid`）：
  grid 列宽按 §四、sticky 表头、行 hover、常显删除、底部加行、编辑态自动补空行。
- 请求参数 / 请求头 / 响应字段 / 请求体字段四处接入；`BodyFieldsEditTable` 改为复用同一套样式。
- 验收：四张表列宽比例与客户端一致（截图对照）；表头在滚动时保持可见；空态为一行灰字；
  编辑态末行输入后自动出现新空行，且未动过的空行不会让保存变脏。

### P2 请求体段对齐（1 天）

- 类型行 + 格式化/复制/JSON 语法提示；form/multipart 的 type 列与只读请求数据示例；
  字段表「生成数据表格」可选，做则必须与 `deriveBodyFields` 同口径。
- 验收：五种类型切回来内容不串（json 文本进 `http_request_body`，form 行保持各自列）；
  `none` + 空表 → 保存清空对应列（现有 `serializeExtEdit` 语义不变）。

### P3 认证 / 文档 / 只读段（0.5 天）

- 认证尺寸与联动、文档工具条、`runtime` 只读段。
- 验收：三种认证类型的字段与该显示的一致；密码/token 不回显明文；只读段没有任何输入框。

### P4 `enabled` 跨仓落库（1–1.5 天，依赖服务端 + 客户端同版本）

- 按 §六 改三仓；补服务端 `sync_test` 与客户端往返用例。
- 验收：Web 勾掉一行 → 保存 → 刷新仍停用 → 客户端拉取后该行 `enabled:false` → 客户端勾回来后 Web 也能看到；
  老数据（无 `enabled` 键）全部按启用展示。

## 九、明确不做

- 右侧常驻响应面板、发送按钮、快捷键、cookie/环境解析、脚本与断言执行。
- 客户端 tab 状态记忆、页内查找、批量编辑（批量编辑可另立需求）。
- 响应头/状态码快照编辑（运行时产物，Web 只回传）。
- 把 `TryItPanel` 改成内嵌右栏（保持弹窗）。
- 为对齐而复制客户端组件（`@/types`、Wails i18n 依赖不可带进 Web；Web 自建同构组件，只对齐契约）。

## 十、涉及文件

| 文件 | 角色 |
|---|---|
| `web/src/views/project/ApiDetailView.vue` | 段序、大纲、保存、`enabled` 序列化、各段编排 |
| `web/src/components/ApiParamTable.vue` | 参数/请求头/响应字段表格（启用列、grid 列宽、sticky） |
| `web/src/components/BodyFieldsEditTable.vue` | 请求体字段表（复用同一套表格样式） |
| `web/src/components/`（新增） | 通用 `RowGrid` / `KeyValueTable`（若抽取） |
| `web/src/utils/api-params.ts` | 头 ↔ 参数行，带 `enabled` |
| `web/src/types/entities.ts` | `ApiParam` / `ApiHeader` 加 `enabled` |
| `web/src/i18n/{zh-CN,en-US}` | 新增列名/空态/停用提示等 key（中英同步） |
| `server/internal/service/bruno.go` | `apiParamRow` / `apiHeaderRow` 加 `Enabled *bool` |
| `client/internal/collection/syncrows.go` | 同步载荷读写 `enabled`（缺省 true） |

## 十一、风险与回滚

| 风险 | 处理 |
|---|---|
| 只上 Web 不上客户端 → 勾选状态被客户端推送抹掉 | 按 §六 发布顺序；先在客户端兼容读，再上 Web 勾选框 |
| 四张表各自实现导致样式再次漂移 | P1 必须先抽公共表格，再逐段接入 |
| sticky 表头被祖先 `overflow` 破坏 | 段容器不加 `overflow`；回归时滚动长表核对 |
| `enabled:false` 行被 `serializeExtEdit` 当空行删掉 | 空判定只看字段名/值，不看 `enabled`；补单测 |
| 段序调整打乱用户习惯 | 大纲与段序同源（`sections`），一次改完；不保留旧顺序开关 |

## 十二、状态

| 阶段 | 状态 |
|---|---|
| 方案（不移植运行时、不 tab 化、竖排对齐） | 定稿 2026-11-05 |
| P0 段落骨架 | ✅ 2026-11-05 |
| P1 表格重写 | ✅ 2026-11-05 |
| P2 请求体段 | ✅ 2026-11-05 |
| P3 认证 / 文档 / 只读段 | ✅ 2026-11-05（文档工具条未加按钮，见下方记录） |
| P4 `enabled` 跨仓 | ✅ 2026-11-05（服务端 + 客户端 + Web 同批落地） |

## 十三、实施记录（2026-11-05）

落地内容与逐条对照：

1. **P0**：`ApiDetailView` 的 `sections` 与 `data-sec` 改为 8 段顺序（请求头 / 请求参数 / 请求体 / 认证 /
   响应字段 / 响应示例 / 变量脚本断言 / 文档；请求头按阅读习惯提到参数之上）；`develop_language` 从独立段
   收进头部胶囊；`activeSec` 起始段为请求头。
2. **P1**：新增全局样式 `web/src/styles/row-grid.css`（`.rg-*`），`ApiParamTable` 与 `BodyFieldsEditTable`
   改用 div-grid、sticky 表头、行 hover、常显操作列、底部加行；参数/请求头加启用勾选；编辑态末尾自动补空行，
   保存前由 `cleanParams` 过滤占位行。
3. **P2**：请求体段改为「类型行（格式化 / 复制 / JSON 语法提示）→ 原文编辑器 → 表单体 grid（含启用、类型列、
   只读请求数据示例）→ 字段表（含「生成数据表格」）」上下堆叠，无子页签。
4. **P3**：认证段改为 560 宽 / 90px 标签列 / 160px 下拉；变量 / 脚本 / 断言合并为只读段；响应内容类型移到
   响应字段段的标题行右侧。**文档段未新增「编辑/生成/下载」工具条**：Web 的编辑入口是顶部统一的「编辑」，
   生成/下载已由导出功能覆盖，再加按钮会出现两个入口，故只对齐了段标题与正文样式。
5. **P4**：`enabled` 契约三仓落地 —— Web `ApiParam`/`ApiHeader` + `api-params.ts`（缺省启用、只在 `false` 时写键）、
   服务端 `apiHeaderRow`/`apiParamRow` 加 `Enabled *bool`（并在 OpenAPI / 契约 / 代码片段导出时过滤停用行）、
   客户端 `syncrows.go` 推送与拉取都保留启用位。回归用例：`web/src/utils/api-params.test.ts`、
   `server/internal/service/export_test.go`、`client/internal/collection/syncrows_test.go`。

6. **接口概述（summary）**：详情页查看态不再「空值整行隐藏」——始终给「添加概要…」入口，
   点文本或占位即进编辑态并聚焦（与客户端 RequestBar 同交互）；公开分享页在 URL 下补一行概要，
   左侧搜索也匹配概要文本。

已知未做（延续原取舍）：右侧响应面板、发送、脚本执行、页内查找、批量编辑、`TryItPanel` 内嵌化；
响应头/状态码快照仍只回传不编辑。
