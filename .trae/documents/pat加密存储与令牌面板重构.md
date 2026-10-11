# PAT 加密存储 + 令牌面板重构（后端加密 / 可查看可删除 / 排版优化）

## Context

现状：PAT 明文只在创建时返回一次，服务端只存 SHA-256 hash；上一版把明文暂存前端 `localStorage`（键 `apidoc.pat.<id>`），导致**换浏览器/清缓存就看不到**。用户要求：
1. PAT 明文**后端加密存储**，前端点眼睛随时查看（任何浏览器都能看）；
2. 调整令牌面板排版（当前挤在一起），并且 PAT 可以**删除**；
3. 创建表单加**字段说明**，操作列文字改为**按钮**。

## 设计要点

- 加密：新增 `internal/pkg/crypto`，AES-256-GCM；密钥 = SHA-256(`JWT_SECRET`)[:32]（不新增配置，文档注明生产须设强 JWT_SECRET）。输出 = base64(nonce+密文)。
- 查看：新增 `GET /api/user/token/reveal?id=`（本人令牌、未吊销才可解密返回明文）。
- 删除：`DELETE /api/user/token?id=` 从「吊销(soft)」改为**硬删除**（`Unscoped().Delete`），前端按钮文案改「删除」，确认后不可恢复。
- 兼容：旧令牌 `token_cipher` 为空 → reveal 返回错误「该令牌未保存明文，请重新创建」。

## 改动清单

### A. 后端
1. 新增 `server/internal/pkg/crypto/crypto.go`：`Encrypt(secret, plaintext string) (string, error)` / `Decrypt(secret, cipher string) (string, error)`（AES-256-GCM，随机 12B nonce 前置，base64；含单测）。
2. `server/internal/model/sync.go` `UserToken` 加 `TokenCipher string \`gorm:"column:token_cipher;size:512;not null;default:''" json:"-"\``。
3. 迁移：
   - `server/migrations/sqlite/0011_token_cipher.up.sql`：`ALTER TABLE user_token ADD COLUMN token_cipher VARCHAR(512) NOT NULL DEFAULT '';`（.down：`DROP COLUMN`）
   - `server/migrations/postgres/0010_token_cipher.up.sql` 同理（postgres 无 0010，沿用 0009 之后编号）。
4. `server/internal/service/token.go`：
   - `CreateToken`：同时写 `TokenCipher = crypto.Encrypt(s.secret, raw)`（保留 TokenHash 供鉴权）。
   - 新增 `RevealToken(ctx, userID, id) (string, error)`：查本人令牌 → 已吊销/密文为空报错 → 解密返回。
   - `RevokeToken` → `DeleteToken`：改调 repo 硬删除。
5. `server/internal/repository/sync.go`：`RevokeUserToken` → `DeleteUserToken`（`Unscoped().Delete`，按 id+userID）。
6. `server/internal/handler/token.go`：新增 `RevealToken` handler（`GET /api/user/token/reveal?id=`），返回 `{token}`。
7. `server/internal/router/router.go` user 组注册 `u.GET("/token/reveal", h.RevealToken)`。
8. 文档：`server/doc/api_docs/sync/token.yml` 补 reveal 方法与 token_cipher 说明；server AGENTS.md「S15 未实现」改为「PAT 已 AES-GCM 加密存储（reveal 查看）」。

### B. 前端
1. `web/src/api/auth.ts`：新增 `revealToken(id)` → `get('/user/token/reveal', { id })`，返回 `{token}`；`revokeToken` 更名 `deleteToken`（仍 DELETE，语义=删除）。
2. `web/src/components/TokenPanel.vue` 重构：
   - **去掉 localStorage**：明文只在会话内存缓存（`ref<Record<number,string>>`），眼睛查看时调用 `revealToken(id)` 拉取并缓存；隐藏不再请求。
   - **创建表单**：改为带 label 的 `n-form`（名称/作用域/有效天数三行或两行排列），每个字段配**说明文字**（`tokenNameHint`/`tokenScopeHint`；天数提示留在 placeholder）。
   - **操作列按钮化**：「复制」「删除」由 text 链接改为小号实体按钮（`type="default"` / `type="error"`），删除走 `dialog.warning` 确认，成功后刷新列表。
   - **排版**：表格行距/单元格内边距加宽、PAT 列放宽；移除已无意义的「最近使用」列（低价值且挤占空间）——列变 6 列：令牌/名称/作用域/有效期/状态/操作。
3. `web/src/components/MyCenterModal.vue`：宽度 680 → 760px。
4. i18n（zh-CN / en-US 一一对应，改完跑 `npm run i18n:check`）：
   - `user.tokenDelete` / `tokenDeleteConfirm` / `tokenDeleteDone`、`tokenRevealFailed`
   - `user.tokenNameHint` / `tokenScopeHint`
   - 保留 `tokenShow/tokenHide` 等既有 key

## 验证

1. 后端（server/）：`go build ./... && go vet ./... && go test ./internal/pkg/crypto/`。
2. 前端（web/）：`npm run i18n:check`、`npx vue-tsc --noEmit`、`npm run lint`、`npm run build`。
3. 端到端：创建令牌 → 查 SQLite `user_token.token_cipher` 为密文（非明文）→ 前端眼睛查看返回明文 → 换浏览器登录后仍可查看（服务端解密）→ 删除令牌后行消失、客户端同步被拒。
