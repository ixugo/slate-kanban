# Slate 前端（web/）

浏览器端单页应用：看板列表、看板画布、设置、Agent 接入说明。**免登录**，与 Go 服务同源部署。

## 技术栈

Vite 8、React 19、TypeScript 7、Tailwind 4、shadcn/ui（vendored 在 `src/components/ui/`）、TanStack Query 5、React Router 7、Vitest。

自研替代（已不依赖对应 npm 包）：`src/i18n`、`src/lib/toast.ts` + `src/components/ui/sonner.tsx`、`src/lib/markdown.tsx`。样式合并用 npm 包 **`cn`**（见 `src/lib/utils.ts`），行为不同于 `tailwind-merge`。

## 常用命令

在仓库根目录：

| 命令 | 说明 |
|------|------|
| `make web/dev` | 开发服务器（默认 `http://localhost:5173`），`/api` 等代理到 `127.0.0.1:7070` |
| `make web/test` | `tsc -b`、`oxlint`、`vitest run` |
| `make web/build` | 构建到 `internal/web/spa/dist` 并跑体积检查 |
| `make build` | 前端构建 + 编译嵌入前端的 Go 二进制 `./slate` |

在 `web/` 目录也可用：`npm run dev|build|test|lint|typecheck|size`。

## 构建产物与体积

- 输出目录：`../internal/web/spa/dist`（gitignore）。
- Go 嵌入：`go build -tags embed_web`（见根目录 `Makefile`）。
- 预算脚本：`scripts/check-size.mjs`（可传 dist 路径作为第 1 参数）。
- 当前门禁：**首屏 JS gzip ≈ 118.5 kB**（上限 130 kB）；**整个 dist gzip ≈ 243.9 kB**（上限 300 kB）。

首屏统计包含 `index.html` 里的入口脚本与 `modulepreload` 的 JS。

## 路由（以 `src/app/AppRoutes.tsx` 为准）

| 路径 | 说明 |
|------|------|
| `/` | 看板列表 |
| `/b/:boardId` | 看板页（懒加载） |
| `/b/:boardId/settings` | 看板设置（懒加载） |
| `/agents` | Agent 接入（懒加载） |
| `*` | 404 |

无登录页；REST 请求不带认证头。

## 目录速览

| 路径 | 内容 |
|------|------|
| `src/app/` | 路由、外框、顶栏控件、错误边界 |
| `src/features/` | 按功能分页（boards、board、card、settings、agents） |
| `src/lib/` | API、主题、toast、错误、Markdown、工具 |
| `src/i18n/` | 多语言运行时与五语 `locales/` |
| `src/components/ui/` | shadcn 基础组件（vendored） |
| `src/test/` | vitest 共用 setup、假 API、假 EventSource |

细节见 `.cursor/scopes/web.app.md`。

## 本地偏好

| 键 | 含义 |
|----|------|
| `slate.lang` | 界面语言（`zh-CN` / `zh-TW` / `en` / `ja` / `ko`） |
| `slate.theme` | `light` / `dark` / `system` |

## 测试

`npm run test` 跑全量 vitest（jsdom）。`src/test/setup.ts` 会预加载五种语言包，并为每个用例安装假的 `EventSource`。

更完整的设计与测试清单见 `.cursor/scopes/web.app.md`。