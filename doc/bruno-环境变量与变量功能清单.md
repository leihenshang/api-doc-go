# Bruno 环境变量与变量功能清单

> 用途：梳理 Bruno（API 客户端）在**环境变量**与**变量设置**方面的全部能力，作为本项目（api-doc-go）后续是否支持/对齐的对照清单。
> 来源：Bruno 官方文档 `docs.usebruno.com`（站点当前为 **v4** 文档，含 v2/v3 版本差异提示），抓取日期 **2026-09-26**。
> 本地实证：`api-sample/bruno/project-A/`（用户导出的 OpenCollection YAML 集合）与 `server/internal/service/bruno.go`（本项目的 Bruno 导入实现）。
> 标注约定：**✔ 文档明确** / **△ 文档未明确（需以实际导出文件或 `bru run -h` 实测）**。

---

## 一、能力总览

| 变量类型 | 作用域 | 持久化位置 | UI 入口 | 脚本读 | 脚本写 |
|---|---|---|---|---|---|
| Runtime 运行时变量 | 本次运行 / 当前集合 | 不落盘（应用本地存储） | 右上角 `(...)` → Variables | `bru.getVar` `hasVar` `getAllVars` | `bru.setVar` `deleteVar` `deleteAllVars` |
| Request 请求变量 | 单个请求 | 请求文件 | 请求 → **Vars** 页签（Pre Request） | `bru.getRequestVar` | 无 setter（走 Vars 页签） |
| Folder 目录变量 | 该目录内请求 | `folder.yml` / `folder.bru` | 目录菜单 → Folder Settings → Vars | `bru.getFolderVar` | 无 setter |
| Collection 集合变量 | 整个集合 | `opencollection.yml` / `bruno.json` | Collection Settings → Variables | `bru.getCollectionVar` `hasCollectionVar` | `bru.setCollectionVar` `deleteCollectionVar` `deleteAllCollectionVars`（落盘） |
| Environment 环境变量 | 当前激活环境 | 集合内 `/environments/<env>.yml` 或 `.bru` | 右上角 Environments | `bru.getEnvVar` `hasEnvVar` `getAllEnvVars` `getEnvName` | `bru.setEnvVar` `deleteEnvVar` `deleteAllEnvVars`（落盘） |
| Global Environment 全局环境变量 | 整个 workspace（跨集合） | workspace `environments/`（3.0.0+；2.x 在应用数据目录） | 右上角 Environments → **Global** 页签 | `bru.getGlobalEnvVar` `hasGlobalEnvVar` `getAllGlobalEnvVars` | `bru.setGlobalEnvVar` `deleteGlobalEnvVar` `deleteAllGlobalEnvVars` |
| Prompt 交互式变量 | 单次请求发送 | **从不存储** | 无（在请求里写 `{{?提示}}`） | 不可访问 | 不可访问 |
| Process 进程环境变量 | 进程 / CI | 独立 `.env` 文件（集合根或 workspace 级） | 由 Bruno 创建管理 | `bru.getProcessEnv(key)` / `process.env` | 只读 |

**变量优先级（高 → 低，作用域由窄到宽）** ✔
```
Runtime > Request > Folder > Environment > Collection > Global
```
- Runtime 优先级最高。
- Prompt（`{{?…}}`）与 Process（`{{process.env.X}}`）**不参与**该优先级栈。
- 环境内继承时先做**环境层合并**（ancestor → parent → active environment），合并结果再进入上面的 scope stack。
- ⚠ **官方两页顺序不一致**：Variables/Overview 为 `Runtime > Request > Folder > Environment > Collection > Global`；Scripting/JavaScript-Reference 写作 `Runtime > Request > Folder > Collection > Environment`（漏 Global，且 Collection 与 Environment 互换）。本清单以 Overview 为准，实际以实测为准。

**存储位置汇总** ✔：集合 → `opencollection.yml`；目录 → `folder.yml`；请求 → `request.yml`；环境 → `/environments/*.yml`；运行时/全局（2.x）→ 应用本地存储；Prompt → 不存储；进程环境 → `.env`。原文强调"All storage is local"。

---

## 二、功能清单（可勾选对照）

### 2.1 变量记录与管理
- [ ] 变量四要素：`名称 / 值 / 描述(可选) / 是否启用(enabled)`
- [ ] 变量类型（v4）：`string / number / boolean / object`，`.bru` 用 `@number` `@boolean` `@object` 注解，YAML 用 `{ type, data }`
- [ ] 值旁的类型下拉选择器（Vars 页签内）
- [ ] 描述支持多行（YAML 块标量 `|-`；bru `@description('''…''')`）
- [ ] 停用（`enabled: false`）而不删除
- [ ] 环境内**搜索变量**（按名称或值实时过滤；选中行会短暂"pin"住便于就地编辑）
- [ ] 空名/空值行 = 孤儿描述行（YAML 兼容）
- [ ] 变量插值语法高亮（编辑器内 `{{var}}` 提示）

### 2.2 作用域与优先级
- [ ] 7 类变量 + 进程环境变量（见总览表）
- [ ] 明确的优先级链并在 UI/文档中可解释
- [ ] 目录变量只对该目录内请求可见
- [ ] 请求变量只对该请求可见（请求级隔离）
- [ ] 环境→全局环境的跨集合共享
- [ ] workspace 隔离（3.0.0+ 每个 workspace 有独立全局环境集）

### 2.3 环境（Environments）
- [ ] 多环境（local / staging / production …），文件即环境（`/environments/<名称>.yml|.bru`）
- [ ] `No environment` 选项 + "未选环境"时 `bru.getEnvVar` 返回 `undefined`
- [ ] 环境**默认值**：Collection → Presets → Save，写入 `bruno.json` 的 `defaultEnvironment` / `opencollection.yml` 的 `extensions.bruno.defaultEnvironment`
- [ ] 默认环境生效规则：有"最近选择"则优先最近选择；否则用默认；CLI 省略 `--env` 时用默认
- [ ] 环境**颜色标记**（3.1.0+）：环境设置内点画笔选色；环境下拉与 tabs 的环境 pill 显示颜色
- [ ] 环境**继承**（4.2.0+）：`extends: <父环境名>`；单父、必须同作用域（集合环境↔集合环境，全局↔全局）、可链式（`STG-GW → STG → base`）
  - [ ] 子环境值**始终优先**；合并顺序 = 最远祖先 → 各祖先 → 直接父 → 当前环境
  - [ ] 合并**在读取时计算，绝不写回磁盘**（磁盘上只有 `extends` 字段）
  - [ ] 父环境的值**可以引用子环境定义的变量**（先展平原始值，再统一插值一次）
  - [ ] 继承来的行**只读并标注来源环境**；用同名本地变量覆盖后该名称转为可编辑
  - [ ] 父环境缺失 → 非阻塞警告（如 *Referenced parent environment(s) not found*），仍用子环境自身变量
  - [ ] 环/多父/自继承被选择器禁止；手工改文件造成环时能提示且不无限循环
  - [ ] 版本约束：`extends` 仅 **4.2.0+** 写入，用更早版本打开会**丢弃该字段**
- [ ] 环境变量与密钥的**导入/导出**：Environments → Configure → Import/Export
  - [ ] 导入来源：Bruno、Postman（页面未提 Insomnia）
  - [ ] 导出：多选环境 → JSON（页面未提 `.bru`/`.env`/`.yml`）
  - [ ] 冲突处理：**Replace / New（加 "Copy"）/ Skip**，支持 Apply to all，Cancel 取消整批
  - [ ] secret 变量**不写入**导出文件
  - [ ] 跨 workspace 迁移（导出→导入）
- [ ] 2.x → 3.0.0 升级：默认 workspace 的全局环境**自动迁移**到其 `environments/`；自定义 workspace 需手工重建/导入

### 2.4 变量插值（模板引擎）
- [ ] `{{变量名}}` 语法，可用于 **URL / 查询参数 / 请求头 / 请求体(JSON、表单) / 认证字段**
- [ ] **递归解析**：变量值里可再引用变量（UI 里直接拼装，无需前置脚本），直到无占位符
- [ ] **类型感知**：
  - [ ] 加引号 `"{{x}}"` → 字符串
  - [ ] 不加引号 `{{x}}` → 保留原生类型（数字/布尔/对象/数组可直接嵌入 JSON 体）
  - [ ] 对象属性 `{{obj.prop}}`、嵌套 `{{obj.a.b}}`、数组下标 `{{arr[0]}}`、成员属性 `{{cfg[0].port}}`（v2.2.0+；不加引号时对象/数组整体序列化为 JSON）
  - [ ] `Date` 值插值为 ISO 字符串（v2.2.0+）
- [ ] 未定义变量**原样保留**（不报错）
- [ ] 循环引用检测：`a→{{b}}`、`b→{{a}}` 时最后一个未解析占位符原样保留
- [ ] △ 字面量 `{{` 的转义机制（文档未提及）

### 2.5 动态变量（Dynamic Variables，Bruno 2.x+）
- [ ] 基于 faker.js，语法 `{{$randomXxx}}`，**区分大小写**（camelCase）
- [ ] 在请求体/参数/请求头/认证等字段**直接可用**；脚本中必须 `bru.interpolate('{{$randomFirstName}}')` 求值
- [ ] 基础：`$guid` `$randomUUID` `$randomNanoId` `$timestamp`(秒) `$isoTimestamp` `$randomAlphaNumeric` `$randomBoolean` `$randomInt`(0–1000) `$randomColor` `$randomHexColor` `$randomAbbreviation` `$randomWord` `$randomWords`
- [ ] 网络：`$randomIP` `$randomIPV4` `$randomIPV6` `$randomMACAddress` `$randomPassword` `$randomLocale` `$randomUserAgent` `$randomProtocol` `$randomSemver` `$randomDomainName` `$randomDomainSuffix` `$randomDomainWord` `$randomExampleEmail` `$randomEmail` `$randomUserName` `$randomUrl`
- [ ] 姓名：`$randomFirstName` `$randomLastName` `$randomFullName` `$randomNamePrefix` `$randomNameSuffix` `$randomJobArea` `$randomJobDescriptor` `$randomJobTitle` `$randomJobType` `$randomPhoneNumber` `$randomPhoneNumberExt`
- [ ] 地理：`$randomCity` `$randomStreetName` `$randomStreetAddress` `$randomCountry` `$randomCountryCode` `$randomLatitude` `$randomLongitude`
- [ ] 图片：`$randomAvatarImage` `$randomImageUrl` + 各分类图片（abstract/animals/business/cats/city/food/nightlife/fashion/people/nature/sports/transport）`$randomImageDataUri`
- [ ] 金融：`$randomBankAccount` `$randomBankAccountName` `$randomCreditCardMask` `$randomBankAccountBic` `$randomBankAccountIban` `$randomTransactionType` `$randomCurrencyCode` `$randomCurrencyName` `$randomCurrencySymbol` `$randomBitcoin`
- [ ] 商业：`$randomCompanyName` `$randomCompanySuffix` `$randomBs*`（`$randomBs`/`$randomBsAdjective`/`$randomBsBuzz`/`$randomBsNoun`）`$randomCatchPhrase*`
- [ ] 数据库：`$randomDatabaseColumn` `$randomDatabaseType` `$randomDatabaseCollation` `$randomDatabaseEngine`
- [ ] 日期：`$randomDateFuture` `$randomDatePast` `$randomDateRecent` `$randomWeekday` `$randomMonth`
- [ ] 文件/系统：`$randomFileName` `$randomFileType` `$randomFileExt` `$randomCommonFile*` `$randomFilePath` `$randomDirectoryPath` `$randomMimeType`
- [ ] 电商：`$randomPrice` `$randomProduct` `$randomProductAdjective` `$randomProductMaterial` `$randomProductName` `$randomDepartment`
- [ ] Hacker/Lorem：`$randomNoun` `$randomVerb` `$randomIngverb` `$randomAdjective` `$randomPhrase` `$randomLoremWord(s)` `$randomLoremSentence(s)` `$randomLoremParagraph(s)` `$randomLoremText` `$randomLoremSlug` `$randomLoremLines`

### 2.6 Prompt 交互式变量
- [ ] 语法 `{{?提示文案}}`（UI 中蓝色高亮），可用于 URL/查询参数/请求头/请求体/认证字段
- [ ] 发送请求时弹模态框，列出所有提示变量，填值后发送；**同名变量只提示一次**，值复用到各处
- [ ] **不持久化**、仅本次执行有效、变量查看器不显示
- [ ] **脚本不可访问**（pre-request / post-response 都不行）
- [ ] 留空 → 替换为空字符串；**取消弹窗 → 请求不发送**
- [ ] CLI / Collection Runner 中此类请求被**跳过并标记 skipped**
- [ ] 典型场景：不写入集合的敏感数据、临时值、发送前必须人工确认的输入
- [ ] 版本：**v4 新增**（v2/v3 无）

### 2.7 进程环境变量与 .env
- [ ] 值中写 `{{process.env.VAR_NAME}}` 引用（如全局环境变量值里放 `{{process.env.API_KEY}}`）
- [ ] `.env` 文件位于集合根目录；**v3.1.0+** 支持在 Bruno 内以 workspace 级别创建与管理
- [ ] `bru.getProcessEnv(key)` 读 OS 级 `process.env`（CI/宿主机注入，**不写入集合文件**）
- [ ] 用于存放 API key/密码等，本地保存、不进版本控制（Bruno 生成的 `.gitignore` 含 `.env*`，本地样例已确认）
- [ ] 不参与变量优先级栈（独立语法，不冲突）

### 2.8 密钥变量与密钥管理
- [ ] 声明：环境变量勾选 `secret`；bru 写法 `vars:secret [ apikey ]`，YAML 写法 `secret: true`
- [ ] 存储：仅本机；优先使用 **OS 级加密**，否则回退 **AES256**
- [ ] **不写入环境文件、不入库**；导出集合/环境时不导出 secret；可安全提交源码管理
- [ ] v4.0.0 起密钥变量在**独立标签页**展示，避免与普通变量混淆
- [ ] Secret Manager 集成：HashiCorp Vault / AWS Secrets Manager / Azure Key Vault / Google Cloud Secret Manager（配置 provider → 拉取密钥 → 在字段中使用）
- [ ] 脚本取密钥：`bru.getSecretVar('<服务名>.<密钥名>')`（需先在集合密钥管理设置里配置）
- [ ] 报告脱敏（Secret Masking）与 CLI 侧密钥（`bru-cli/secret-managers`、`--secrets-env-file`）
- [ ] 迁移：`secrets.json` → 环境文件（secrets-management/migration）

### 2.9 脚本与运行时
- [ ] 脚本时机：**pre-request / post-response / tests**；作用层级：集合、目录、请求
- [ ] bru 格式 tag：`script:pre-request { }`、`script:post-response { }`、`tests { }`（官方 tag 参考页此处存在文档不一致——标题写 `test`、示例写 `body:test`，以实测为准）；另有 `meta { tags: [...] }` 供运行时按标签筛选
- [ ] OpenCollection YAML：`runtime.scripts[] = { type: before-request | after-response | tests, code: |- … }`（本地样例与该页结构一致）
- [ ] 请求级 **Vars 页签**：Pre Request 可写任意字符串/数字/合法 JS 字面量；Post Response 可写任意 JS 表达式并可用 `res`，用于**声明式**解析响应并赋值
- [ ] 变量 API 全清单（见 §3.4）
- [ ] `bru.interpolate(str)`：脚本里求值含变量/动态变量的字符串
- [ ] OAuth2 凭据变量：`bru.getOauth2CredentialVar(key)`、`bru.resetOauth2Credential(id)`
- [ ] **v4 破坏性变更**：所有写入/删除 API **默认持久化到磁盘**，`{ persist: true }` 选项已移除（依赖"仅内存"行为的脚本升级后表现不同）

### 2.10 CLI（`bru run`）
- [ ] `--env <名称>` 指定运行环境
- [ ] `--env-var <覆盖表达式>` 覆盖单个**集合级**环境变量（可多次使用）
- [ ] `--global-env <名称>` 指定全局/工作区级环境（使用 `--global-env-var` 的前置条件）
- [ ] `--global-env-var <覆盖表达式>` 覆盖单个全局环境变量（可多次）
- [ ] `--env-file <.bru|.json>` 指定本次运行使用的环境文件
- [ ] `--secrets-env-file <dotenv>` 外部密钥管理器凭据文件；运行期注入、**不落盘、不进日志**，需与 `--env` 同用
- [ ] `bru docs generate --all-envs | --envs a,b | --exclude-envs a,b` 生成 HTML 文档时选择环境（**排除优先于包含**）
- [ ] △ `--env-var` / `--global-env-var` 的取值书写格式（如是否 `KEY=VALUE`）
- [ ] △ `--cwd` 不存在（文档为 `--workspace-path`）；`--reporter` 不存在（为 `--reporter-json/-junit/-html`）

### 2.11 文档与协作相关
- [ ] 生成 HTML 文档时可嵌入环境并切换（`html-docs/environments`；本次未展开细节）
- [ ] 集合导出/默认环境随集合走（`bruno.json` / `opencollection.yml`）
- [ ] 环境可跨 workspace 单独导出/导入

---

## 三、关键机制细节

### 3.1 环境文件两种格式

**.bru（`environments/local.bru`）**
```bru
extends: dev
vars {
  @description('''Base URL for the local server''')
  host: http://localhost:8787

  @description('''
  JWT token obtained after login. Expires in 24 hours.
  ''')
  token: abc123
}
vars:secret [
  apikey
]
```

**YAML（`environments/production.yml`）**
```yaml
name: Production
extends: dev
variables:
  - name: host
    value: https://api.example.com
    enabled: true
    secret: false
    description: Single-line description
  - name: apiKey
    value: ''
    enabled: true
    secret: true
    description: Secret API key stored encrypted
  - name: disabledVar
    value: old-value
    enabled: false
    secret: false
```

> 全局环境文件（workspace `environments/*.yml`）的条目还带一个 `type: text` 字段（官方该页仅出现这一种类型值）。

**类型化变量（v4）**
```bru
vars:pre-request {
  @number timeout: 30
  @boolean debug: true
  @object config: '''
  { "host": "localhost", "port": 8080 }
  '''
  baseUrl: https://api.example.com
}
```
```yaml
runtime:
  variables:
    - name: timeout
      value: { type: number, data: "30" }
    - name: config
      value: { type: object, data: '{"host":"localhost","port":8080}' }
```

### 3.2 默认环境配置落盘位置
```json
// bruno.json
{ "version": "1", "name": "my-api", "defaultEnvironment": "Local" }
```
```yaml
# opencollection.yml
opencollection: 1.0.0
info:
  name: my-api
extensions:
  bruno:
    defaultEnvironment: Local
```

### 3.3 本地样例实证（`api-sample/bruno/project-A`）
- 集合根：`opencollection.yml`（`opencollection: 1.0.0` + `info` + `config` + `bundled` + `extensions.bruno.ignore`）
- 目录：`用户/folder.yml`（`info.type: folder` + `request.auth: inherit`）
- 请求：`用户/用户-列表.yml`（`info` / `http`（`method` `url` `params[].type=query` `auth`）/ `settings` / `docs`）
- 样例中 URL/说明里已存在 `{{Token}}`、`https://{{服务器地址}}` 形式的引用，但**集合未附带 `environments/` 目录**
- `.gitignore` 由 Bruno 生成，内容含 `.env*`、`node_modules`、`.DS_Store` 等

### 3.4 变量相关脚本 API（精确清单）

| 作用域 | 方法 |
|---|---|
| Runtime | `bru.setVar(k,v)` `bru.getVar(k)` `bru.hasVar(k)` `bru.getAllVars()` `bru.deleteVar(k)` `bru.deleteAllVars()` |
| Request（只读） | `bru.getRequestVar(k)` |
| Folder（只读） | `bru.getFolderVar(k)` |
| Collection | `bru.getCollectionVar` `bru.hasCollectionVar` `bru.setCollectionVar` `bru.deleteCollectionVar` `bru.deleteAllCollectionVars` `bru.getCollectionName()` |
| Environment | `bru.getEnvName()` `bru.getEnvVar` `bru.setEnvVar` `bru.hasEnvVar` `bru.getAllEnvVars` `bru.deleteEnvVar` `bru.deleteAllEnvVars` |
| Global Environment | `bru.getGlobalEnvVar` `bru.setGlobalEnvVar` `bru.hasGlobalEnvVar` `bru.getAllGlobalEnvVars` `bru.deleteGlobalEnvVar` `bru.deleteAllGlobalEnvVars` |
| Process（只读） | `bru.getProcessEnv(k)` `bru.cwd()` |
| OAuth2 凭据 | `bru.getOauth2CredentialVar(k)` `bru.resetOauth2Credential(id)` |
| Secret Manager | `bru.getSecretVar('<service>.<key>')` |
| 插值工具 | `bru.interpolate(str)` |
| 数据驱动 | `bru.runner.iterationData.has/get/stringify/unset`、`bru.runner.iterationIndex`、`bru.runner.totalIterations`（数据文件**不会自动替换请求体**，需 `{{列名}}` 或 `req.setBody()`） |

要点：`req` / `res` **本身没有变量读写方法**，一切变量读写都在 `bru.*`；常见组合是 `bru.setVar("token", res.getBody().token)` 与 `req.setHeader("Authorization", "Bearer " + bru.getEnvVar("access_token"))`。`bru.setEnvVar` 会按值**自动推断类型**并写入对应注解。

### 3.5 版本差异（v2 / v3 / v4）

| 能力 | v2 | v3 | v4（当前） |
|---|---|---|---|
| Prompt 变量（`{{?…}}`） | 无 | 无 | 有 |
| OpenCollection YAML 格式 | 无 | 有 | 有 |
| 全局环境存于 workspace `environments/` | 无（应用数据目录） | 有（3.0.0+） | 有 |
| 全局环境继承 / 环境继承 `extends` | 无 | 无（4.2.0 起） | 有 |
| 类型化变量 `@number/@boolean/@object` | 无 | 无 | 有（**破坏性**：旧版无法解析） |
| 环境颜色标记 | 无 | 有（3.1.0+） | 有 |
| 密钥独立标签页 | 无 | 无 | 有 |
| Google Secret Manager / CLI secret-managers | 无 | 无 | 有 |
| 变量写入 API 是否落盘 | 可由 `{ persist }` 选项控制 | 可由 `{ persist }` 选项控制 | **默认落盘**（`{ persist: true }` 选项已移除） |

---

## 四、与本项目（api-doc-go）的现状对照

`server/internal/service/bruno.go` 当前只做"请求 → 接口"的静态导入：
- 只解析 `info.type == "http"` 的文件；`folder.yml` → 接口分组；`opencollection.yml`、点号开头文件跳过
- **环境变量、脚本等非请求文件按代码注释"按设计跳过"**（`req.Info.Type != "http"` 即跳过，不计失败）
- URL / 请求头里的 `{{var}}` 原样落库，**不做变量解析**；项目内没有环境、变量、密钥、Vars 等概念
- 因此：Bruno 变量体系中**当前被忽略**的部分 = §2.3 环境、§2.4 插值的运行时行为、§2.5 动态变量、§2.6 Prompt、§2.7 进程环境、§2.8 密钥、§2.9 脚本与 Vars

若后续要对齐，建议优先考虑（按投入产出排序，仅作选项，不代表已排期）：
1. **环境条目入库**：导入 `environments/*.yml|.bru` 为项目级"环境"记录（名称/变量表/enabled/描述/secret 标记），文档页提供环境切换后的 URL 变量预览
2. **变量表与插值展示**：接口详情里对 `{{var}}` 高亮并可选环境解析出示例 URL，保留原值不改写
3. **动态变量说明页**：把 §2.5 清单作为只读参考表放进文档/帮助页
4. **默认环境**：读取 `opencollection.yml` 的 `extensions.bruno.defaultEnvironment` 作为导入时的默认环境
5. 密钥变量仅记录"存在"与名称，**不导入值**（Bruno 本身也不把 secret 写进文件）

---

## 五、待核实清单（本次文档未明确，需实测或查证）

| # | 问题 | 建议核实方式 |
|---|---|---|
| 1 | `.bru` 请求文件中 post-response vars 的 tag 写法（官方 tag-reference 页未收录 `vars:*`） | 用 Bruno 桌面端建一个带 Post Response Vars 的请求后查看生成文件 |
| 2 | YAML **请求文件**是否存在**声明式**变量字段（structure-reference / samples 均只出现 `runtime.scripts`，未出现 `variables` 段） | 同上，对比导出文件 |
| 3 | YAML **集合文件 / folder 文件**中变量容器的确切字段名（官方仅给 `runtime.variables` 形式的 YAML 片段，未给集合级示例） | 导出带集合变量与目录变量的集合核对 |
| 4 | `--env-var` / `--global-env-var` 的取值格式与和 `--env-file` 的完整优先级 | `bru run -h` 实测 |
| 5 | 全局环境是否随集合导出（文档未明说；存于 workspace 目录，倾向不随集合） | 导出集合后检查产物 |
| 6 | 生成 HTML 文档时环境切换的具体表现 | `html-docs/environments` 页 + `bru docs generate` 实测 |
| 7 | 字面量 `{{` 的转义写法 | 官方文档未提及，实测插值行为 |

---

## 六、参考来源（官方文档页）

变量：`/variables/overview` `/variables/environment-variables` `/variables/global-environment-variables` `/variables/collection-variables` `/variables/folder-variables` `/variables/request-variables` `/variables/runtime-variables` `/variables/prompt-variables` `/variables/process-env` `/variables/interpolation`
密钥：`/secrets-management/overview` `/secrets-management/secret-variables` `/secrets-management/dotenv-file` `/secrets-management/secret-masking` `/secrets-management/secret-managers/*`
脚本：`/testing/script/overview` `/testing/script/javascript-reference` `/testing/script/vars` `/testing/script/dynamic-variables` `/testing/script/script-flow`
环境导入导出：`/get-started/import-export-data/import-export-environments`
CLI：`/bru-cli/commandOptions` `/bru-cli/runCollection` `/bru-cli/secret-managers`
格式：`/bru-lang/tag-reference` `/opencollection-yaml/structure-reference` `/opencollection-yaml/samples`
HTML 文档：`/html-docs/environments`
