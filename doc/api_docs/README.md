# api_docs 接口文档

- 格式：对齐 `_example/` 的请求式 yml（`info/http/settings/docs`），**一接口一文件**，按模块目录组织。
- `common/`：统一响应、错误码、分页与鉴权约定（对接必读）。
- 生成来源：`doc/项目开发规范.md` §5；契约实现以 `server/internal/dto/dto.go` 与 handler 为准，接口变更需同步更新对应 yml。
