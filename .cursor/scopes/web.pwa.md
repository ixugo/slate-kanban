# 渐进式 Web 应用（PWA）：可安装、离线只读

## 需求目标

- 桌面/手机可把 Slate 安装成独立应用（standalone）。
- 离线时能打开上次的看板界面，数据为最近一次在线时的快照（只读，不能改）。

## 问题场景

| 编号 | 问题场景 | 不解决的后果 | 解决方案 | 状态 |
|------|----------|-------------|---------|------|
| 1 | 浏览器不支持或用户用 http + 局域网 IP 访问 | 误以为能离线，实际 SW 不注册 | 仅在 `https` 或 `localhost` 注册 SW；README 写明限制 | 实施 |
| 2 | 离线打开只有白屏 | 无法演示「上次看板」 | 预缓存应用壳（`index.html`、`theme-init.js`、图标、manifest）；导航网络优先，失败回退 `/`；成功拿到 HTML 时只更新 `/` 这一份，不按地址（带任务、搜索参数的 URL）堆积缓存，也不让非 HTML 响应覆盖壳 | 实施 |
| 3 | 离线看不到任务 | 只有壳没有数据 | SW 对 `GET /api/v1/boards` 与 `GET /api/v1/boards/:id/snapshot` 网络优先并缓存成功响应 | 实施 |
| 4 | 登录失效或改了口令，离线仍能看到旧看板 | 已失去授权的内容还留在浏览器里 | 服务端返回 401 时清空整个只读 API 缓存，既不写入也不回退旧数据 | 实施 |
| 5 | MCP、SSE、写接口、非网页地址被缓存 | 行为错乱，离线时 `/install.md` 之类的地址会被替换成网页壳 | `/mcp`、`* /stream`、`Accept: text/event-stream`、`/api/v1/auth/`、非白名单 API，以及 `/install.md`、`/skills/`、`/health`、`/app/` 都不经 SW 缓存策略 | 实施 |
| 6 | 缺失的 `sw.js` 被当成 HTML | 浏览器拒绝注册 SW | Go `spa` 对 `/sw.js`、`/manifest.webmanifest`、`/icons/*` 禁止 SPA 回退，真 404 | 实施 |
| 7 | 用户不知道离线只能看 | 反复点保存失败 | 全局 `OfflineBanner` + `pwa.offlineBanner` 五语言文案 | 实施 |
| 8 | 离线时还能拖任务、点新建 | 操作后才失败并回滚，像出了 bug | 离线时看板页禁用拖动、不显示「添加任务」（看板与列表视图都是），恢复联网自动回来；任务详情接口不缓存，离线点开详情，弹窗如实提示连不上服务器（只有服务端明确返回 404 才说任务已不存在） | 实施 |
| 9 | 只在看板视图下用过的人，离线时切到列表视图加载失败 | 列表视图的代码是按需加载的，离线时下载不到 | 打开看板 1.5 秒后空闲预取列表视图的代码（`prefetchListView`），SW 的静态资源缓存会存下它 | 实施 |
| 10 | 共用设备上退出登录后，离线缓存里还留着上一个人的看板 | 下一个人离线也能看到 | 顶栏退出按钮给 SW 发 `clear-caches` 消息（`pwa/clearCaches.ts`），SW 删除全部 `slate-*` 缓存 | 实施 |

## 方案

- `web/public/manifest.webmanifest`：`standalone`，主题色取自 `index.css` 浅色背景 `#f7f7f9`、品牌近黑 `#1d1d1f`。
- 图标：`scripts/make-icons.mjs`（Playwright + 系统 Chrome）从 `favicon.svg` 生成 192/512/maskable/apple-touch。
- `web/public/sw.js`：手写三桶缓存（shell / assets / api），`CACHE_VERSION` 升级清旧桶；`clear-caches` 消息清空全部 `slate-*`。
- 前端：`register.ts` 仅生产+安全上下文注册；`useOnlineStatus` + `OfflineBanner` 挂 `App.tsx`；`clearCaches.ts` 在退出登录时让 SW 清缓存。
- Go：`handler.go` 增加 `.png` MIME；`noSpaFallback` 保护 SW/manifest/icons。

## 验证计划

1. `cd web && npx vitest run src/pwa`：hook、Banner、注册、SW 路由（vm 假环境，含壳只存 `/` 一份、非网页地址直连、401 清缓存）；`src/features/board/BoardOffline.test.tsx`：离线时看板不能拖、不显示新建入口，断网和恢复联网即时生效。
2. `go test ./internal/web/spa/`：manifest/png MIME、no-cache、缺失 SW/图标 404。
3. `npm run build` 通过 `check-size.mjs`。
4. 真机（Playwright）：`Page.getInstallabilityErrors` 无错误；在线打开看板 → 离线刷新仍有列和任务 + 提示条；恢复在线提示消失；离线 POST 不破坏缓存；离线时切列表视图正常、没有添加入口、拖动不会移动任务、点开详情提示连不上服务器。

## v1 不做

| 项 | 说明 |
|----|------|
| 离线编辑/队列同步 | 只读快照，写操作仍失败 |
| 搜索/筛选离线 | 仅缓存 snapshot，不缓存 search |
| iOS「添加到主屏幕」全量验收 | 以 Chromium 安装性与离线为主 |