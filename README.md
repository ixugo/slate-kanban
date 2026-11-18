# Slate

Slate 是一个**人和 AI agent 共用的看板**：人在网页上拖卡片、写需求，agent 通过 MCP 读看板、领任务、干活，做完把卡片交到「审查」列等人检查。

- **单文件、默认免登录、本机优先**：一个可执行文件（前端已嵌进去，gzip 传输）加一个 SQLite 文件，没有 per-user 账号系统；可选配置一个共享的访问口令（见「安全说明」）。
- **人和 agent 看同一份数据**：网页和 MCP 读写的是同一个看板，任何一边改了，另一边立刻收到更新；agent 离线时错过的变更，上线后可以一次补读。
- **只处理「进行中」**：人把卡片拖进 agent 的领取列（默认是「进行中」），agent 才会去领；做完自动交到下一列（默认是「审查」）。
- **Trello 式自动化**：卡片创建、进入某列、离开某列时触发规则，自动写开始/完成时间、改优先级、加标签、留言、移动到别的列……
- **五种界面语言**：简体中文、繁体中文、English、日本語、한국어，深浅主题；看板可在画布视图与排序表格列表视图之间切换。
- **干净的界面**：胶囊按钮、毛玻璃顶栏、弹簧动效；系统开了「减少动效」「减少透明度」「高对比度」时自动适配。「完成」列默认收在右侧的窄边里，点一下展开，收起状态记在浏览器里。

> 设计取向是「先用起来，再用它自己管理自己的开发」：Slate 的开发任务和开发记录就放在它自己的看板里。

## 快速开始

### 方式一：本机编译运行

需要 Go 1.27+、Node 22+ 和 make。

```sh
make build   # 安装前端依赖（第一次）、构建前端、编译成带前端的单文件程序 ./slate
./slate      # 启动，浏览器打开 http://127.0.0.1:7070
```

首次启动会在当前目录生成 `configs/config.toml`（默认配置）和 `configs/data.db`（数据库），日志写在 `logs/`。
相对路径都以**启动时所在的目录**为基准。想换配置目录：`./slate -conf /path/to/configs`。

### 方式二：Docker

```sh
docker compose up -d --build   # 然后打开 http://127.0.0.1:7070
```

数据库和日志在命名卷 `slate-data` 里，重建容器不会丢。`docker-compose.yml` 默认只把端口映射到宿主机的 `127.0.0.1`。

### 开机自启

在项目目录里（要和手工 `./slate` 时同一目录，这样数据库和 `logs/` 路径一致）：

```sh
./slate service print                    # 预览当前系统的服务文件（也可 --platform launchd|systemd）
./slate service install [-conf ./configs]   # 写到当前用户目录，不自动加载
./slate service uninstall                # 只删除服务文件，不自动停服务
```

`install` 只会生成文件：macOS 写到 `~/Library/LaunchAgents/dev.slate.kanban.plist`，Linux 写到 `~/.config/systemd/user/slate.service`。**程序不会替你执行 `launchctl` 或 `systemctl`**，终端里会打印接下来要手工跑的命令，例如：

- **macOS**：`launchctl bootstrap gui/<uid> <plist路径>`，再 `launchctl kickstart -k gui/<uid>/dev.slate.kanban`。用户级 LaunchAgent 在你**登录之后**才会拉起，不是未登录时的系统级开机自启。日志可在项目 `logs/slate-service.stdout.log` 查看。
- **Linux**：`systemctl --user daemon-reload`，再 `systemctl --user enable --now slate.service`。若希望**没人登录也在开机后运行**，还需一次 `loginctl enable-linger <你的用户名>`。日志用 `journalctl --user -u slate.service -f`。

卸载前先按提示 `launchctl bootout` 或 `systemctl --user disable --now`，再 `./slate service uninstall`。可用 `install --dry-run` / `uninstall --dry-run` 只看会写什么、删什么。

### 第一个项目

1. 打开网页，点「新建项目」，填名称，再用开关选起步模板：
   - **研发流程**（开关打开，默认）：想法池 → 待办 → 进行中 → 审查 → 完成，带三条默认规则：进入「进行中」时写开始时间、进入「完成」时写完成时间（已有的时间不覆盖）、离开「完成」时清除完成时间。
   - **空白**（开关关闭）：不建列，到「设置 → 列」里自己加。
   - 网页只提供这两种；接口和 MCP 创建项目时还可以用 `simple`（待办 → 进行中 → 完成）。
2. **项目路径**网页上不用填，也看不到：agent 第一次接入时会问你「这个项目对应当前仓库吗」，你确认后由它写入（`set_project_path`），agent 据此创建 worktree 干活。写错了需要用管理视图 MCP 的 `update_board` 或 REST 接口改。
3. 在「Agent」页复制安装命令，把 MCP 和 skill 装进你的 agent（见下一节）。

## 让 agent 接入

Slate 对 agent 提供三样东西：**MCP 服务**（读写看板）、**skill**（教 agent 怎么按看板的规矩干活）、**安装说明**（让 agent 自己把前两样装好）。

最省事的做法：把下面这句话发给你的 agent（地址换成你的实际地址）：

```text
读 http://127.0.0.1:7070/install.md ，按说明完成 Slate 的接入。
```

也可以在网页的「Agent」页按 host 选择 Claude Code、Cursor 或 Codex，复制现成的命令或配置。以 Claude Code 为例：

```sh
# 工作视图：agent 日常领任务、干活（16 个工具）
claude mcp add --transport http --scope user slate 'http://127.0.0.1:7070/mcp?name=claude-code'

# 管理视图：建项目、改列、写规则、录入任务（23 个工具；项目只能创建，不能通过 MCP 删除），只有需要时再装
claude mcp add --transport http --scope user slate-manager 'http://127.0.0.1:7070/mcp?name=claude-code-manager&role=manager'
```

装完后在 agent 里**新开一个会话**，让它调用 `whoami` 确认身份（已经打开的会话不会获得新装的 MCP 和 skill）。也可以先在终端检查：Claude Code 用 `claude mcp list`，Cursor 命令行用 `agent mcp list`，Codex 用 `codex mcp list`（它只列出配置，不会真的连服务）。启用口令后漏带口令时各 host 的表现、Cursor 需要重启或批准等细节，都写在服务的 `/install.md` 里。

### 身份怎么确定

Slate 没有登录，身份由 MCP 地址里的参数决定：

| 地址 | 身份 | 能做什么 |
| --- | --- | --- |
| `/mcp?name=<名字>` | agent（工作视图） | 读看板；领取、释放、交接自己领的卡片；评论；建卡；写项目路径 |
| `/mcp?name=<名字>&role=manager` | 人（管理视图） | 以上之外，还能建/改/删看板、列、规则，删卡片和评论，修改优先级、依赖、截止时间等规划字段 |

`name` 只能用小写字母、数字、`.`、`_`、`-`，以 字母或数字开头，最长 64 位；`human`、`system` 是保留名。不同 agent 请用不同的名字，评论、领取、更新游标都按名字区分。

### agent 怎么干活

skill 里写明的流程：

1. 第一次接触某个看板时，确认它的**项目路径**；没填就问你，填了就不再问（`set_project_path`）。
2. 用 `wait_for_updates` 阻塞等待（或按固定间隔 `pull_updates`），有新变更再醒来；离线期间的变更靠游标补读，不会丢。
3. 只处理**领取列**（默认「进行中」）里的卡片：`next_card` 取下一张，`claim_card` 领取（租约默认 30 分钟，最长 24 小时，干活时自动顺延，超时自动归还）。
4. 读完卡片的描述、评论、依赖，以及**这一列的 agent 指令**（列上写的做事规则）；在独立分支或 worktree 里开发，复杂任务派 subagent。
5. 做完调用 `finish_card`：写一段总结、填分支名和链接，卡片自动进入下一列（默认「审查」）；需要人拍板就留言并 `release_card`。
6. 审查没通过时，人在卡片下留言并把卡片拖回「进行中」，agent 再来领，按留言继续改。

## 自动化规则（Trello 式，但更简单）

在「设置 → 自动化」里给每个项目配置规则。大多数时候不用自己写：

- **推荐规则**：还没添加的推荐会列在下面（进入「进行中」写开始时间、进入「完成」写完成时间、离开「完成」清除完成时间），点「添加」就生效。
- **新建规则**像说两句话：「当 [卡片进入 / 卡片离开 / 新建卡片] [某一列或任意列]，就 [动作]，并且 [动作]……」。没自定义名字时，规则名按内容自动生成。
- 要改名字、让时间动作「总是覆盖」、或加筛选条件时，再展开「更多选项」。

规则的组成：

| 部分 | 可选项 |
| --- | --- |
| 触发器 | 卡片创建时 `card_created`、进入某列时 `card_entered`、离开某列时 `card_left` |
| 条件 | 可选，用下面的搜索语法描述（例如 `tag:紧急 -is:human`） |
| 动作 | 写/清开始时间、写/清完成时间、设/清截止时间、设优先级、加/去标签、设「仅人工」、加评论、移动到某列（共 12 种） |

规则可以连锁（一条规则把卡片移到别的列会继续触发那里的规则，最多 3 层，同一条规则一次操作只执行一次）。每条规则最多 20 个动作，每个项目最多 50 条规则。
列还有两个「驱动 AI」的开关：**agent 领取列**（agent 在这里领卡片）和**完成列**（进入这里的卡片算已完成），以及一段**给 agent 的指令**——agent 领到这一列的卡片时必须遵守。

## 搜索语法

网页搜索框和 MCP 的 `list_cards` 共用同一套语法，空格分隔、全部满足才命中，词前加 `-` 表示取反：

```text
col:进行中 is:free -is:waiting        # 进行中、没人领、没在等依赖的卡片
tag:后端 priority:>=2                 # 带「后端」标签、优先级中及以上
is:overdue -is:done                   # 已逾期且没完成
claimed:claude-code                   # 被 claude-code 领着的卡片
due:<2026-12-01 no:branch             # 12 月前到期、还没有分支名
tag:bug|ui col:待办|进行中              # 竖线表示多选一：带 bug 或 ui 标签，且在待办或进行中
```

前缀有 `tag` `col` `key` `title` `desc` `priority` `claimed` `is` `has` `no` `due` `created` `updated`。完整说明见官网的「筛选语法」页。
网页上除了搜索框，旁边的「筛选」按钮可以点选标签、列、优先级、领取者、状态和截止日期，选好的条件会自动拼成上面的语法。
网页快捷键：`/` 搜索、`n` 新建卡片、`Esc` 关闭或清空。

## 安装为应用（PWA）

在 **https** 或本机 **localhost** 打开 Slate 时，浏览器可提供「安装应用」；安装后以独立窗口运行（`manifest.webmanifest` + Service Worker）。

**限制：**

- Service Worker 只在安全上下文生效。若用局域网 IP 的 **http** 访问，无法安装，也无法离线打开。
- 离线时只能查看**上次在线打开时**已缓存的看板快照（列表 + 单板 `snapshot`）；所有修改、搜索、实时流、MCP 均不可用，页面顶部会显示离线提示。卡片详情没有缓存，离线时点开只会提示连不上服务器。

图标可由 `cd web && node scripts/make-icons.mjs` 从 `favicon.svg` 重新生成。

## 导出

- 网页：看板工具栏的「导出」菜单。
- 接口：`GET /api/v1/boards/:id/export?format=json|md`，JSON 含看板、列、规则、全部卡片和评论；Markdown 是方便阅读和归档的文档。

## 配置

`configs/config.example.toml` 是带注释的完整样例，常用项：

| 配置 | 默认值 | 说明 |
| --- | --- | --- |
| `Server.HTTP.Host` | `127.0.0.1` | 监听地址。**默认只允许本机访问**，要给别的机器用请先读下面的「安全说明」 |
| `Server.HTTP.Port` | `7070` | 端口 |
| `Server.HTTP.AllowedHosts` | `[]` | 额外允许的 Host（不含端口）；IP、`localhost`、`*.localhost` 始终允许。通过域名访问时必须写进来 |
| `Server.HTTP.Passcode` | `''`（空） | 可选访问口令，空表示不启用。只能是可打印 ASCII（不含空格），最长 128 位，不合规启动失败；非空时 REST、`/mcp`、`/app` 要带 `Authorization: Bearer <口令>`，网页则先登录 |
| `Data.Database.Dsn` | `./configs/data.db` | SQLite 文件路径；以 `postgres` 开头则按 PostgreSQL 连接 |
| `Log.*` | `./logs`、`info` | 日志目录、级别、保留天数、分割大小 |

## 安全说明

**默认没有登录：未配置口令时，任何能访问到端口的人都可以读写全部看板。** 所以：

- 默认只监听 `127.0.0.1`，只有本机能访问。
- 服务端有三道兜底：Host 白名单（防 DNS 重绑定，通过域名访问需配置 `AllowedHosts`）、写请求的 Origin 必须和 Host 一致（防跨站请求）、固定的安全响应头（禁止被嵌入 iframe 等）。
- **可选访问口令**：在 `configs/config.toml` 里设置 `Server.HTTP.Passcode`（只能是可打印 ASCII、不含空格，建议至少 8 位，最长 128 位）。启用后 `/api/`、`/mcp`、`/app/` 都要先通过口令：
  - 网页：先出现登录页，输入口令后浏览器保存 `slate_session` 会话 cookie（HttpOnly、SameSite=Strict、30 天，实时流也靠它）；顶栏有退出按钮，退出时同时清掉离线缓存；会话中途失效会自动回到登录页。
  - MCP、REST、agent：`Authorization: Bearer <口令>`。启用口令后，`/install.md` 里三个 host 的接入示例会带上口令占位。
  - 仍然公开：`/health`、`/install.md`、`/skills/…`、`/api/v1/auth/*` 和网页的静态文件（登录页本身要能打开）。
  - 同一个来源 IP 每分钟猜错口令超过 10 次，会被短暂拒绝（429）。限流按直连 IP 计，放在反向代理后面时所有人共用代理的地址，会互相影响。
  - 局限：整个服务只有一个共享口令，知道口令的人等同管理员；服务只讲 http，局域网里口令是明文传输，要防窃听请放在 https 反向代理后面；退出登录只清浏览器里的 cookie，要让已发出的会话全部失效请改口令；性能分析接口（pprof）由自己的 IP 白名单保护，不受口令控制。
- 要让局域网或公网使用，可以配置口令，也可以放在**带认证的反向代理**后面，并把对外域名写进 `AllowedHosts`；不要无保护地把端口暴露出去。
- MCP 的身份（`name`）是自报的，只用来区分 agent 和记录谁做了什么，不是安全边界；口令只负责进门，不替代 `name`。

## 开发

```text
.
├── main.go                      入口：读配置、启动服务
├── internal/
│   ├── core/kanban/             看板领域：看板、列、卡片、评论、领取与租约、事件、规则引擎、导出
│   │   └── stores/kanbandb/     数据库存储（GORM + SQLite / PostgreSQL）
│   ├── web/api/                 REST（/api/v1）与路由装配、安全中间件
│   │   ├── kanbanapi/           看板、卡片、规则、事件流、导出
│   │   └── connectapi/          接入说明（/install.md、/skills/…、/api/v1/connect）
│   ├── web/mcpapi/              MCP 服务（/mcp）：两套工具视图、通知、内嵌 skill
│   ├── web/spa/                 嵌入前端产物并提供 gzip 静态服务
│   ├── svcinstall/              开机自启：生成并安装 launchd / systemd 服务文件（./slate service）
│   └── conf/、app/、data/       配置、依赖装配（Wire）、数据库
├── web/                         前端：React 19 + Vite + Tailwind 4 + TanStack Query + shadcn/ui
├── skills/slate-kanban/         agent 用的 skill（编译进程序，也可从服务下载）
├── configs/                     配置样例
└── .cursor/scopes/              每个功能的设计文档和需求质量清单
```

后端基于 [goddd](https://github.com/ixugo/goddd) 的分层（API → Core ← Store，Wire 装配）。常用命令：

| 命令 | 作用 |
| --- | --- |
| `make run` | 启动后端（开发模式，不带前端） |
| `make web/dev` | 启动前端开发服务器 `http://127.0.0.1:5173`，接口和 MCP 转发到本机 7070 |
| `make web/test` | 前端类型检查、oxlint、单元测试 |
| `make web/e2e` | 浏览器端到端冒烟测试：自动构建前端、编译并启动临时服务，用本机 Chrome 跑用例（约 30 秒；需要装 Chrome，不碰 `configs/` 里的真实数据） |
| `make web/build` | 构建前端（产物进 `internal/web/spa/dist`），并检查体积预算 |
| `make build` | 前端 + 后端，得到带前端的 `./slate` |
| `go test ./internal/...` | 后端测试（默认 SQLite，不依赖 docker） |
| `make test-postgres` | 用一次性 docker PostgreSQL 跑看板存储层测试（`internal/core/kanban/stores/kanbandb`） |
| `go vet ./internal/... ./pkg/... .` | 后端静态检查 |
| `make docker/build` | 用多阶段 Dockerfile 构建镜像 |

前端体积有预算：首屏 JS gzip 后不超过 130 kB，整个 `dist` gzip 后不超过 300 kB，`npm run build` 会检查。
前端产物不进仓库：`go build` 默认不嵌入前端（访问网页会看到提示），加 `-tags embed_web` 才嵌入，`make build` 已经带上。

## 文档

- 设计文档与需求质量清单：`.cursor/scopes/`（每个功能一份，含流程图、决策表、测试说明和代码对照）。
- 官网（使用指南、设计取舍、MCP 工具、REST 接口、部署）：同级目录 `../slate_web`（Docusaurus），`npm install && npm start` 本地预览。
- 开发记录：`docs/devlog/`，已经用 MCP 录入 Slate 自己的看板。

## 进度与路线

已完成（v1）：

- [x] 看板、列、卡片、评论、依赖、标签、优先级、截止时间、仅人工
- [x] 搜索语法、拖拽排序（乐观更新）、键盘操作、实时同步（SSE）、导出 JSON / Markdown
- [x] 领取与租约、完成交接、事件日志与游标、离线补读、阻塞等待
- [x] Trello 式自动化规则（3 种触发器、12 种动作、连锁、条件）
- [x] MCP 两套视图、通知、内嵌 skill、一句话接入
- [x] 免登录与安全兜底、gzip 静态服务、单文件部署、Docker
- [x] 五种语言、深浅主题、官网
- [x] 新建项目只要名称和一个模板开关；完成列可收起；自动化有推荐规则和「两句话」编辑器；苹果风格的界面
- [x] 列表视图（排序表格）、活动流（工具栏「活动」抽屉）、卡片磁贴可在新标签页打开
- [x] PWA：可安装，离线时能查看上次缓存的看板快照
- [x] 可选访问口令（`Server.HTTP.Passcode`）、开机自启（`./slate service`）、英文官网（入门、概念、接入已译，其余章节暂显示中文）
- [x] PostgreSQL 存储层测试（`make test-postgres`）、浏览器端到端冒烟测试（`make web/e2e`）
- [x] 接入已在 Claude Code 和 Codex 的真实会话里走通；Cursor 只验证到 MCP 连接这一层（用它自带的命令行客户端），skill 目录和带大模型的会话还没有验证

以后再做（都已记入 Slate 自己的看板）：

- [ ] 可选的用户名（多人区分评论作者）
- [ ] git 同步、自动更新

## 许可

见 [LICENSE](LICENSE)。
