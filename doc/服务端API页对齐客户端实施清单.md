# 服务端 API 页对齐客户端实施清单

> 目标：把 Web「文档」工作台里的 API 详情页做成客户端 `RequestEditor` 那种 tab 式形态。
> 基线：2026-11-05，`web/src/views/project/ApiDetailView.vue`（纵向长页，10 个 section）。
> 对照：`api-doc-go-client/frontend/src/components/RequestEditor.vue`（9 个 tab）。

结论先行：**信息架构可以对齐，运行时会话层不能照搬**。照搬会让 Web 变重，而且做不到（Web 没有本地 HTTP 引擎）。

---

## 一、现状差异

| 维度 | 客户端 `RequestEditor` | Web `ApiDetailView` |
|---|---|---|
| 信息架构 | 9 个 tab：参数 / 请求体 / 请求头 / 认证 / 响应字段 / 变量 / 脚本 / 断言 / 文档 | 纵向长页 10 段：headers / params / body / auth / result / sample / settings / script / info / doc |
| 顶部 | URL 栏 + 「发送」 | URL 只读 + 调试（弹窗）/ 历史 / 设置 / 代码 / 导出 |
| 响应 | 右侧常驻面板（发送后填充） | 底部「响应」段（文档字段）+ 调试弹窗 |
| 行交互 | 每行有启用勾选、行尾删除、「添加一行」 | 参数表可编辑，但**没有启用列** |
| tab 状态 | 组件内 state | 无（长页滚动） |
| 保存 | 本地文件自动保存 | 整体保存 + `expected_update_time` 乐观锁 |

客户端 tab 定义（`RequestEditor.vue:173-181`）：

```
params / body / headers / auth / resp / vars / script / tests / docs
```

每项带 `dot` 标记（该 tab 是否有内容）。gRPC 另有 `schema / message / metadata / options`。

---

## 二、实施步骤（4 期，每期可独立上线）

### P0 信息架构 tab 化（1–1.5 天）

1. `ApiDetailView` 顶部加 tab 条，映射现有 section：

   | tab | 对应 section | 说明 |
   |---|---|---|
   | 请求参数 | `data-sec="params"` | |
   | 请求体 | `data-sec="body"` | |
   | 请求头 | `data-sec="headers"` | |
   | 认证 | `data-sec="auth"` | |
   | 响应 | `data-sec="result"` + `data-sec="sample"` | 合并「响应字段」与「示例」，共享一个 tab |
   | 脚本 | `data-sec="script"` | |
   | 文档 | `data-sec="doc"` | |

   接口设置已在齿轮弹窗（`data-testid="detail.settings"`），不进 tab。

2. tab 走路由 `?tab=body`，刷新 / 深链接不丢；**切 tab 只切视图，不入库**。

3. tab 上的「有内容」小圆点：复用已有 computed（`hasParams / hasBody / hasHeaders / hasAuth / hasResp / hasScript`）。

4. 空 tab 显示引导文案（对齐客户端的「点这里添加」）。

**验收**：深链接 `?api=69&tab=body` 直接落在请求体 tab；刷新不回到第一个 tab；切 tab 不产生保存请求。

### P1 表格交互对齐（1 天）

1. `ApiParam` 增加 `enabled`（默认 `true`），参数 / 请求头 / 响应字段表格首列加勾选。
2. 行尾删除 + 底部「添加一行」统一（请求体字段表已具备）。
3. 保存时把 `enabled` 写进 `http_request_header / http_request_params / http_return_params / http_request_body_fields` 的 JSON。

**前置**：先查清服务端这几列的 JSON 契约（见问题点 2）。

**验收**：勾掉一行 → 保存 → 刷新后勾选状态保持；客户端同步下来仍能识别。

### P2 右栏响应面板（2 天，改动最大）

1. `TryItPanel`（现在是弹窗）拆成可内嵌组件，右侧常驻。
2. 布局：`内容区 1fr + 响应区 380~480px`；窄屏（<1200）折叠成抽屉。
3. 响应区复用现有发送链路，不新增接口。

**建议**：默认收起，用户主动展开才占宽。否则正文被压缩，写文档反而更难受。

### P3 变量 / 脚本编辑（按需，2 天+）

1. 变量 tab：`{{var}}` 插入器 + 高亮（`VarText` 已有只读版）。
2. 脚本 tab：Web 无执行环境，只能编辑不能验证 —— **建议保持只读**。

---

## 三、问题点（按风险排序）

1. **运行时能力不可移植**。客户端的「发送 + 右侧响应 + 快捷键」依赖本地 HTTP 引擎（cookie jar、环境变量、脚本执行）。Web 的调试走服务端代理，跨域 / 内网接口才发得出去 —— 这点必须保持，不能因为搬 UI 改成浏览器 fetch。

2. **`enabled` 是数据模型改动**。现在 `ApiParam { fieldName, type, required, description, example }` 没有启用位，而客户端同步上来的 `http_request_params` 是 `KV { name, value, enabled }` —— 两套结构在服务端怎么兼容的，必须先查清再动，否则保存会把客户端字段写坏。

3. **tab 化会掩盖「保存」语义**。客户端切 tab 不影响草稿（本地文件即时存）；Web 若切 tab 不保存就丢改动。必须定死：切 tab = 只切视图，右上角「保存」是唯一入口（这条刚在上一轮统一过，别因为 tab 化又散开）。

4. **跨 tab 的必填校验会变难**。整体保存能一次校验所有字段；tab 化后保存失败，报错信息属于另一个 tab 时用户看不到 → 需要「保存失败自动跳到出错 tab」。

5. **响应字段与响应示例是两块数据**。多示例走独立接口（`api_return_sample`），塞进同一个「响应」tab 时要合并两个数据源，否则又会出现「两个保存入口」。

6. **样式对齐的成本在细节**。客户端表头 `sticky`、行 hover 才显示删除、列宽用 `grid`（名称 1.2fr / 值 1fr / 说明 1.4fr）。Web 用 `n-table`，列宽会随内容跳动；要一致就得改 div-grid（请求体字段表 `BodyFieldsEditTable` 已是这种写法）。

7. **组件不能直接搬**。客户端 `BodyFieldsTable / ResponseFields` 依赖 `@/types` 与 Wails i18n，硬搬会带无关依赖。Web 侧自建同构组件，只对齐视觉与交互契约。

8. **窄屏**。客户端是桌面应用，Web 要兼容 1280 甚至 1024。右栏常驻响应在 1280 下会把正文挤到 ~700px，必须做断点降级。

---

## 四、建议取舍

- **做 P0 + P1**：tab 化 + 启用列，视觉立刻接近客户端，风险可控，不改运行模型。
- **P2 谨慎**：右栏响应做成「可收起侧栏」，默认收起（保持现在的弹窗），用户主动展开才占宽。
- **P3 不做**：脚本 / 断言在 Web 编辑了也无法执行，纯增加维护面；真要改就在客户端改。

---

## 五、涉及文件

| 文件 | 角色 |
|---|---|
| `web/src/views/project/ApiDetailView.vue` | 主战场：tab 容器、各 section 渲染、整体保存 |
| `web/src/components/ApiParamTable.vue` | 参数 / 请求头 / 响应字段表格（P1 加启用列） |
| `web/src/components/BodyFieldsEditTable.vue` | 请求体字段表（已是 div-grid） |
| `web/src/components/TryItPanel.vue` | 调试面板（P2 拆成可内嵌） |
| `web/src/types/entities.ts` | `ApiParam` 加 `enabled`（P1） |
| `server/internal/service/content.go` | 保存接口对 `enabled` 的落库（P1 前置确认） |

---

## 六、状态

| 阶段 | 状态 |
|---|---|
| 分析 | ✅ 2026-11-05 |
| P0 tab 化 | ⬜ 待排期 |
| P1 启用列 | ⬜ 待排期（依赖 JSON 契约确认） |
| P2 右栏响应 | ⬜ 待评估 |
| P3 变量 / 脚本 | ⬜ 暂不做 |
