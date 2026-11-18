# 嵌入网页与 gzip 静态服务（internal/web/spa）

## 需求目标

生产构建用 `-tags embed_web` 将 `web` 产物打进二进制；开发/测试无标签时不嵌入，`go test` 不必先 `npm run build`。
同一端口：已注册路由优先，其余 GET/HEAD 走 SPA；API 类未匹配路径仍 JSON 404。

## 行为

- 路径穿越防护；仅 GET/HEAD，其它 405
- 无文件且非 API 路径 → `index.html`（前端路由）
- API 类：`/api/`、`/mcp`、`/health`、`/install.md`、`/skills/`、`/app/` → 不 SPA 回退
- 可压缩类型预 gzip（BestCompression）内存缓存；`ETag` + `If-None-Match` 304
- `/assets/*`：`Cache-Control: public, max-age=31536000, immutable`；其它 `no-cache`
- 无 `index.html` 时 `/` 返回嵌入说明文案 200

## 装配

`internal/web/api/provider.go` 的 `NoRoute(spa.NoRouteHandler(spa.ContentFS()))` 在 `setupRouter` 之前注册。

## 构建

```sh
cd web && npm run build   # 输出 internal/web/spa/dist
go build -tags embed_web .
```