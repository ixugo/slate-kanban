# 嵌入网页与 gzip 静态服务（internal/web/spa）

## 需求目标

生产构建用 `-tags embed_web` 将 `web` 产物打进二进制；开发/测试无标签时不嵌入，`go test` 不必先 `npm run build`。
同一端口：已注册路由优先，其余 GET/HEAD 走 SPA；API 类未匹配路径仍 JSON 404。

## 行为

- 路径穿越防护；仅 GET/HEAD，其它 405
- 无文件且非 API 路径 → `index.html`（前端路由）
- API 类：`/api/`、`/mcp`、`/mcp/`（含下面的子路径）、`/health`、`/install.md`、`/skills/`、`/app/` → 不 SPA 回退，未匹配时 JSON 404
- 可压缩类型预 gzip（BestCompression）内存缓存；`ETag` + `If-None-Match` 304
- 网页静态文件之外的响应由 `internal/web/api/gzip.go` 的 `gzipMiddleware` 压缩：客户端带 `Accept-Encoding: gzip`，类型是文本类（JSON、text/*、SVG 等），响应体不小于 1 KiB，状态 200 且没有 `Content-Encoding`、`Content-Range`。PNG、JPEG、WebP 等已压缩的图片、`text/event-stream` 事件流、HEAD、分段响应都原样返回。看板和卡片列表是几百 KB 的 JSON，压缩后体积约降到十分之一。
- `/assets/*`：`Cache-Control: public, max-age=31536000, immutable`；其它 `no-cache`（含 `/sw.js`、`/manifest.webmanifest`）
- `.webmanifest` → `application/manifest+json`；`.png` → `image/png`
- `/sw.js`、`/manifest.webmanifest`、`/icons/*` 缺失时 **404**，不回退 `index.html`（见 `web.pwa.md`）
- `/.well-known/*` 一律 **404**，不回退 `index.html`：MCP 客户端（Claude Code、Codex）收到 401 或检查连接时，会到这里和 `/mcp/.well-known/*` 找授权元数据，拿到 HTML 会按 JSON 解析失败，Claude Code 会报 `SyntaxError`，看不出是口令的问题（见 `agent.skill.go.md` 的「真实 host 里发现的差异」）
- 无 `index.html` 时 `/` 返回嵌入说明文案 200

## 装配

`internal/web/api/provider.go` 的 `NoRoute(spa.NoRouteHandler(spa.ContentFS()))` 在 `setupRouter` 之前注册。

## 构建

```sh
cd web && npm run build   # 输出 internal/web/spa/dist
go build -tags embed_web .
```