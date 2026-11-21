# 可选访问口令（internal/web/api）

## 需求目标

局域网共享 Slate 时，可在配置里设置一个访问口令：不设置保持免登录；设置后网页靠会话 cookie、MCP 和 REST 靠 `Authorization: Bearer <口令>` 进门。REST 仍固定记在 human 名下，MCP 仍自报 `name`，口令只负责「进门」，不是账号体系。

| 编号 | 问题场景 | 不解决的后果 | 解决方案 | 状态 |
|------|----------|-------------|---------|------|
| 1 | 端口暴露到局域网 | 任何人等同管理员 | `Server.HTTP.Passcode` 非空时，全局中间件拦 `/api/`、`/mcp`、`/app/` | 已实施 |
| 2 | 浏览器的 EventSource 不能自定义请求头 | 实时流带不了 Bearer | 登录后发 `slate_session` cookie（HttpOnly、SameSite=Strict、30 天） | 已实施 |
| 3 | 改口令或重启后旧会话怎么办 | 行为不确定 | 会话是无状态的 HMAC 签名，密钥由口令派生：重启不影响，改口令后所有旧会话失效 | 已实施 |
| 4 | 有人暴力猜口令 | 被撞出口令 | 按 TCP 对端 IP 限流：每分钟猜错超过 10 次，该 IP 返回 429；最多跟踪 1024 个 IP | 已实施 |
| 5 | 限流被绕过 | 形同虚设 | 限流用 `RemoteIP()`，不用会采信 `X-Forwarded-For` 的 `ClientIP()`（那个头客户端可以随便写） | 已实施 |
| 6 | 日志泄露口令 | 口令进了日志文件 | 登录接口不记请求体；日志只记 IP、路径，不记 Authorization 和 cookie | 已实施 |
| 7 | agent 不知道要带口令 | MCP 连不上 | `/install.md` 和 `/api/v1/connect` 在启用口令时，三个 host 的示例都带上口令占位（只传「是否启用」，不传口令本身） | 已实施 |
| 8 | 口令里有空格、中文 | 网页能登录，但 MCP 客户端放不进请求头 | 启动时校验：只允许可打印 ASCII（不含空格），最长 128 位，不合规直接退出 | 已实施 |
| 9 | 网页怎么进门 | 浏览器打开页面时没法自己带 Bearer | `AuthGate` 包住整个应用：`status` 接口说「需要口令且没通过」才显示登录页；还没读到时显示加载条；读状态失败（比如断网）时放行，保留离线只读 | 已实施 |
| 10 | 会话中途过期，或服务端刚启用口令 | 页面一直报错，不知道要重新登录 | 任何接口返回 401 就重新读一次 `status`，它说要登录才切到登录页（地址不变，登录后回到原页面）；同时多个请求一起 401 只发一次状态请求；`status` 自己返回 401（前面有别的网关）时不再重读，防止死循环 | 已实施 |
| 11 | 共用设备上退出登录 | 下一个人离线还能看到上一个人的看板 | 顶栏退出按钮（仅启用口令且已登录时出现）：`POST /logout` 清 cookie，清空内存里的查询缓存，并给 Service Worker 发 `clear-caches` | 已实施 |
| 12 | 登录页被猜口令 | 无 | 登录页对 401 提示「口令不对」，对 `ErrTooManyAttempts`（429）提示稍后再试，两者都有五语言文案；空口令不发请求 | 已实施 |

## 方案

- 配置：`internal/conf` 的 `Server.HTTP.Passcode`；`main.go` 读完配置立刻调 `ValidatePasscode`，`NewHTTPHandler` 里调 `WarnShortPasscode`（短于 8 位只警告）。
- 中间件：`passcodeMiddleware` 按路径前缀判断。受保护：`/api/`、`/mcp`、`/app/`。公开：`/health`、`/install.md`、`/skills/`、`/api/v1/auth/*`，以及网页静态文件和 SPA 外壳。
- 凭证检查只有一个入口 `passcodeGuard.authenticate`，中间件和 `status` 接口共用，规则：
  1. 有效会话 cookie 直接通过（靠签名防伪，猜不出来，所以不限流，浏览器留着过期 cookie 也不会把自己锁死）。
  2. 带 Bearer 才算在猜口令：先看该 IP 是否被封（封锁期内正确口令也拒绝），再比对，错了记一次失败。
  3. 没带凭证或只有无效 cookie：401，不计失败次数。
- 接口：`GET /api/v1/auth/status` 返回 `{required, authenticated}`；`POST /api/v1/auth/login {passcode}` 校验后种 cookie；`POST /api/v1/auth/logout` 清 cookie。这三个接口都不要求口令，但和 `/api/v1` 一样限制请求体大小（1 MiB）和 JSON 类型。
- 口令比较：先各取 SHA-256 再做恒定时间比较，避免长度和内容的时序泄露。
- 网页：`web/src/features/auth/`（`useAuth.ts` 读状态/登录/退出、`AuthGate.tsx`、`LoginPage.tsx`、`LogoutButton.tsx`）；`lib/queryClient.ts` 的全局错误处理负责 401 → 重读状态；`pwa/clearCaches.ts` 通知 SW 清缓存。登录页按需加载，不占首屏。
- 接入说明：`connectapi.New(passcodeRequired bool)`；Claude Code 用 `--header`（必须写在地址后面，否则被当成请求头值吞掉），Cursor 用 `headers`，Codex 用 `--bearer-token-env-var`（口令放环境变量，不写进配置文件）。

## 已知局限（都是有意取舍）

- 只有一个共享口令，没有 per-user 账号；知道口令的人等同管理员。
- 服务本身只讲 http：局域网明文传输口令，要防窃听请放在 https 反向代理后面（此时 cookie 会按 `X-Forwarded-Proto` 自动加 `Secure`）。
- 会话是无状态的，登出只清浏览器里的 cookie，不能单独吊销某一个已发出的 cookie；要让所有会话立刻失效，改口令即可。
- 限流按 TCP 对端 IP 计数：放在反向代理后面时，所有请求都来自代理地址，会被合并计数（一个人猜错多了，大家都会被短暂拒绝）。
- 表满时按「最早出现」淘汰 IP 记录，大量来源 IP 的分布式猜测可以挤掉旧记录，使被挤掉的 IP 提前解封。
- pprof 由自己的 IP 白名单（默认只有本机）保护，不受口令控制。

## 验证

| 项 | 命令/做法 |
|----|----------|
| 单元（口令、限流、会话） | `go test -race ./internal/web/api -run 'Passcode|Attempt|Bearer|Stale|Valid|Status|Login|Session'` |
| 真实路由装配 | `go test ./internal/web/api/passcodeflow`（独立包，因为路由装配只能在一个进程里做一次） |
| 配置校验 | `go test ./internal/conf` |
| 接入说明 | `go test ./internal/web/api/connectapi` |
| 网页（单元与集成） | `cd web && npx vitest run src/features/auth src/lib/queryClient.test.ts src/pwa/clearCaches.test.ts src/app/AppShell.test.tsx`：门的各种状态、登录成功/口令不对/被限流/空口令、401 重读状态（含网关场景和不循环）、退出清缓存 |
| 网页变异检查 | 去掉 401 处理、去掉状态码判断、去掉变更缓存的处理、去掉 `cancelRefetch: false`、去掉对状态接口自身的豁免（会死循环）、退出时不清内存、不通知 SW，对应测试都能失败 |
| 网页真机 | 系统 Chrome 驱动启用口令的服务：首屏是登录页；错口令提示不对；对口令后进入，cookie 为 HttpOnly + SameSite=Strict，`document.cookie` 读不到；刷新仍登录；看板实时流带 cookie 返回 200；清掉 cookie 后在页面里新建任务，不刷新就跳回登录页且地址不变，重新登录回到同一看板；退出后 `slate-*` 缓存从 3 个变成 0、cookie 消失；连续猜错 11 次出现限流提示 |
| 变异检查 | 逐个去掉：口令中间件、`RemoteIP`、失效 cookie 不计数、登录体限制、status 限流、有效 cookie 放行、Bearer 比较，测试都能失败 |
| 真机 | 编译出独立二进制、启用口令后用 curl 跑完整矩阵：公开路径 200、受保护 401、`/mcp` 带 `WWW-Authenticate`、Bearer 通过、登录 cookie 通过、失效 cookie 连发 30 次仍是 401、换 `X-Forwarded-For` 猜 11 次被封 429、封锁期内有效 cookie 仍通过、日志里没有口令 |
