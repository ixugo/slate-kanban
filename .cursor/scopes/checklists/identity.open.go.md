# identity.open.go.md 验收清单

- [ ] 无 `internal/core/access`、`accessapi`、令牌表迁移
- [ ] REST `/api/v1` 无认证，操作者 `human`
- [ ] MCP `?name=` / `&role=manager`，非法 400，保留名 400
- [ ] Host 白名单全路径含 `/health`；Origin 写拦截；安全响应头
- [ ] `/connect` JSON 无 `token_env`，host.mcp 含 worker/manager kind
- [ ] 默认监听 `127.0.0.1`，`AllowedHosts` 可配置