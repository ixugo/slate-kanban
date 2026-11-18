# 免登录与开放身份（internal/web）

## 需求目标

v1 不要账号体系：默认免登录、网页打开即用；可选配置 `Server.HTTP.Passcode` 后网页登录 + Bearer 进门。REST 所有写操作仍记在人名下（固定 `human`）；MCP 用网址参数自报身份与视图。
已删除令牌表、管理令牌配置和 `/agent-tokens`（`internal/core/access` 与 `accessapi` 包都已不存在）。安全靠默认只监听本机、Host 白名单、跨站写拦截、安全响应头——能连上端口的人等同管理员。

## MCP 身份规则

| 参数 | 含义 |
|------|------|
| `name` | agent 名称，缺省工作视图 `agent`、管理视图 `manager`；格式 `^[a-z0-9][a-z0-9._-]{0,63}$`；保留 `human`、`system` |
| `role=worker` 或缺省 | 工作视图（agent 工具集，操作者类型 agent） |
| `role=manager` | 管理视图（23 个工具，操作者类型 human） |

SDK 仍走 Bearer 占位 + `headerVerifier` 读 `X-Slate-Actor-*` 头，会话按创建时 `role` 选 Server。

## 安全兜底

1. **监听地址**：默认 `127.0.0.1`（`config.example.toml` 与 `default.go`）。
2. **Host 白名单**（`Server.HTTP.AllowedHosts`）：非 IP 字面量、非 `localhost` / `*.localhost` 且不在配置列表 → 403；**含 `/health`**，防 DNS 重绑定。
3. **Origin 拦截**：非 GET/HEAD/OPTIONS 且带 `Origin`，主机与 `Host` 不一致 → 403。
4. **响应头**：`X-Frame-Options: DENY`、`X-Content-Type-Options: nosniff`。

## 已知局限

能访问到端口且没有启用口令的人等同管理员。启用口令后只是「进门」，仍无 per-user 账号。对外开放建议结合口令或反向代理认证，并配置 `AllowedHosts`。

## 实施位置

| 能力 | 包/文件 |
|------|---------|
| REST 固定人 | `internal/web/restactor` |
| Host/Origin/头 | `internal/web/api/security.go` |
| 可选口令 | `internal/web/api/passcode.go`、`auth.go` |
| MCP 查询参数 | `internal/web/mcpapi/middleware.go` |
| `/connect` 需口令时与 REST 同保护；安装页仍公开 | `internal/web/api/connectapi/api.go` |
| 全局 Host/Origin | `internal/web/api/security.go` |