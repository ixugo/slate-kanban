# identity.open.go.md 验收清单

- [x] 路由与 Wire 未挂载 `accessapi`；业务不依赖令牌表（仓库里或仍留 `internal/core/access` 死代码，运行路径不经过）
- [x] REST `/api/v1` 无认证，操作者 `human`（`restactor.Middleware`）
- [x] MCP `?name=` / `&role=manager`，非法 400，保留名 400
- [x] Host 白名单全路径含 `/health`；Origin 写拦截；`X-Frame-Options`、`nosniff`
- [x] `/connect` JSON 无 `token_env`，host.mcp 含 worker/manager `kind`
- [x] 默认监听 `127.0.0.1`，端口默认 7070；`AllowedHosts` 可配置