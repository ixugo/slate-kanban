# web.static.go.md 验收清单

- [ ] `embed_web` / `!embed_web` 双构建通过
- [ ] gzip、ETag 304、`/assets/*` 缓存头
- [ ] `/b/*` SPA 回退；`/api/v1/nope` JSON 404
- [ ] 无 dist 时 `/` 友好说明；API 不受影响