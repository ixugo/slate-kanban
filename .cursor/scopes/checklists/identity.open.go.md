# identity.open.go.md 验收清单

- [x] `internal/core/access`、`accessapi` 已删除，路由与 Wire 不再有令牌相关代码，业务不依赖令牌表
- [x] REST `/api/v1` 无认证，操作者 `human`（`restactor.Middleware`）
- [x] MCP `?name=` / `&role=manager`，非法 400，保留名 400
- [x] Host 白名单全路径含 `/health`；Origin 写拦截；`X-Frame-Options`、`nosniff`
- [x] `/connect` JSON 无 `token_env`，host.mcp 含 worker/manager `kind`
- [x] 默认监听 `0.0.0.0`，端口默认 7070；`AllowedHosts` 可配置