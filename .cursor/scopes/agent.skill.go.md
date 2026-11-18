# Agent 接入：skill 与安装说明（skills、internal/web/api/connectapi、internal/web/mcpapi）

## 需求目标

agent 光有 MCP 工具还不够：它得知道怎么用——项目路径问不问、没活怎么等、来活了谁去做、在哪里改代码、做完交给谁。
这些流程写在一个 skill（`slate-kanban`）里；服务端负责让用户和 agent 都能一步装好 MCP 和这个 skill，并且 skill 永远和服务端是同一个版本。

| 编号 | 问题场景 | 不解决的后果 | 解决方案 | 状态 |
|------|----------|-------------|---------|------|
| 1 | agent 只有工具，不知道工作流程 | 领不该领的卡、直接在主分支上改、做完不交接 | 随服务发布 `slate-kanban` skill，写清：先确认看板和项目路径、守候、领取、worktree、子 agent、交接到审查、出错怎么办 | 实施 |
| 2 | 项目在本机的位置没人告诉 agent | agent 不知道去哪个目录建 worktree，或者每次都问用户 | 路径记在看板上（`project_path`），不在本地另存：看板上有就直接用，没有才问用户一次，再用 `set_project_path` 写回；agent 只能写还是空的，已有的只有人能改 | 实施 |
| 3 | 来活之后主 agent 自己干 | 上下文被代码细节占满，一次只能做一张，卡片租约到期 | 主 agent 只做调度：领卡、派子 agent、续租、交接；每张卡一个子 agent、一个 worktree、一个分支；同时进行的卡片数有上限（默认 2）。host 没有子 agent 能力时，主 agent 自己在 worktree 里做，一次一张 | 实施 |
| 4 | 不同 host 有的能长时间阻塞调用，有的不能 | 要么 agent 空转轮询，要么错过新活 | skill 给出选择：优先用 `wait_for_updates` 阻塞守候；工具调用超时短就缩短 `timeout_seconds`；host 不能保持会话就用它自己的定时功能周期性检查 | 实施 |
| 5 | 装 MCP 和 skill 要翻各 host 的文档，令牌还容易写进被提交的文件 | 接入门槛高，令牌泄露 | `/install.md`（免认证）写好 Claude Code、Cursor、Codex 的安装步骤，地址用请求里的；令牌一律走环境变量 `SLATE_TOKEN`，不写进文件；网页的"连接 Agent"用 `/api/v1/connect` 取同一份结构化说明，不在前端另写一套命令 | 实施 |
| 6 | skill 和服务端版本对不上 | skill 里提到的工具不存在或已改名，agent 去调用不存在的工具 | skill 内嵌在服务二进制里，由服务提供：`/skills/slate-kanban/SKILL.md` 和 MCP 资源 `slate://skills/slate-kanban`；测试保证 skill 提到的工具和错误名都真实存在，agent 视图的每个工具 skill 里都有 | 实施 |
| 7 | agent 连上 MCP 时还没装 skill | 不知道还有流程说明 | 连接时下发的 `instructions` 提示：没装就读取资源 `slate://skills/slate-kanban`，经用户同意后存成自己的 skill | 实施 |
| 8 | 免认证的地址被用来做文章 | 通过 Host 头注入内容，或被缓存污染 | 只返回静态内容和从请求里算出的服务地址；Host 和转发头严格校验，不合格返回 400；说明页不缓存；里面没有任何令牌 | 实施 |

## 两条主线

### 安装

```mermaid
sequenceDiagram
    participant H as 人
    participant W as 网页
    participant A as agent
    participant S as 服务

    H->>W: 在 Agents 页创建令牌
    W-->>H: 令牌明文（只显示一次）和一句提示词
    H->>A: 粘贴提示词：请阅读 {服务地址}/install.md 并安装
    A->>S: GET /install.md（免认证）
    S-->>A: 按这个 host 的安装步骤，地址已经替换好
    A->>A: 令牌放进环境变量 SLATE_TOKEN，装 MCP
    A->>S: GET /skills/slate-kanban/SKILL.md（免认证）
    A->>A: 存进自己的 skills 目录
    A->>S: MCP whoami
    S-->>A: name 是令牌名称，kind 是 agent
```

没有走安装页的 agent 也能自救：它连上 MCP 后，`instructions` 提示去读资源 `slate://skills/slate-kanban`，再经用户同意存成自己的 skill。

### 工作

```mermaid
flowchart TD
    A[whoami 确认身份] --> B[list_boards 找看板]
    B --> C{看板有 project_path 吗}
    C -->|有| E[get_board 读每一列的指令]
    C -->|没有| D[向用户确认一次项目目录]
    D -->|确认| D2[set_project_path 写回看板] --> E
    D -->|不确认| Z[不开工]
    E --> F{有空闲名额吗}
    F -->|没有| W
    F -->|有| G[next_card 或 list_cards + claim_card]
    G -->|有卡片| H[建 worktree 和分支, 派子 agent]
    G -->|没有卡片| W[wait_for_updates 阻塞等待]
    H --> I[comment_card 续租和汇报]
    I --> J[子 agent 返回, 检查分支]
    J --> K[finish_card 交接到下一列]
    K --> F
    W --> F
    H -.->|做不下去| R[comment_card 说明, release_card]
    R --> F
```

## skill 的内容结构

`skills/slate-kanban/SKILL.md`，开头是 `name` 和 `description`（host 据此决定什么时候加载它），正文按顺序：

| 节 | 内容 | 对应需求 |
|----|------|---------|
| 先确认能用 | `whoami` 能返回 agent 身份才算装好；没装去读 `/install.md` | 5、6 |
| 找到看板和项目路径 | 按当前仓库匹配看板；路径为空问一次并写回；已有路径不再问；路径和当前目录不一致先问用户 | 2 |
| 守候和领取 | 阻塞循环；并发上限；单张用 `next_card`，多张用 `list_cards` 加 `claim_card`；按 host 能力选守候方式；重启后用 `claimed:` 找回自己持有的卡片 | 3、4 |
| 派发 | 读全文和评论；建 worktree 和分支；子 agent 的约束和汇报要求；每 20 分钟评论续租；`finish_card` 交接；返工 | 3 |
| 做不下去 | 提问、释放、新任务记在非领取列、规划类字段不能改；服务端错误对照表 | 1 |
| 收尾 | 卡片完成且分支已合并后清理 worktree 和分支 | 3 |

关键规则：agent 只处理 `agent_pickup` 为 true 的列（模板里叫「进行中」）；做完用 `finish_card`，不自己把卡片拖进完成列；不改人的规划类字段。
这些规则服务端本来就会强制（agent 做不到会得到对应的错误），skill 的作用是让 agent 一开始就不去尝试。

## 服务端提供的东西

| 路径 | 认证 | 内容 |
|------|------|------|
| `GET /install.md` | 无 | Markdown 安装说明：令牌、三种 host 的 MCP 安装命令、skill 安装命令、验证方法 |
| `GET /skills/slate-kanban/SKILL.md` | 无 | skill 全文，文件和仓库里的 `skills/slate-kanban/SKILL.md` 是同一份（编译时内嵌） |
| `GET /api/v1/connect` | 令牌 | 同一份说明的 JSON 形式，网页的"连接 Agent"用它渲染带复制按钮的代码块 |
| MCP 资源 `slate://skills/slate-kanban` | 令牌 | skill 全文，两个视图都有 |

免认证的两个地址不返回任何敏感信息：skill 是使用说明，安装说明里令牌的位置只是占位符。它们免认证是因为 agent 在拿到令牌、装好 MCP 之前就需要它们。

### 服务地址怎么算

`/install.md` 和 `/api/v1/connect` 要把服务地址写进命令里。地址从请求里算，零配置，用户用什么地址打开页面，命令里就是什么地址：

| 部分 | 来源 | 校验 |
|------|------|------|
| 协议 | `X-Forwarded-Proto` 的第一个值，没有就看连接是否是 TLS | 只认 `http` 和 `https`，别的值忽略 |
| 主机 | `X-Forwarded-Host` 的第一个值，没有就用 `Host` | 只允许字母、数字、`-`、`.`、`_`，可以带 `:端口`，IPv6 用方括号；总长不超过 255；不合格返回 400 |

这些头都是调用者自己发的，响应只回给他自己，并且说明页带 `Cache-Control: no-store`，所以不会出现别人看到被污染内容的情况。
反向代理放在子路径下（例如 `/slate/`）的部署暂不支持，见"限制"。

### 结构化说明（`/api/v1/connect`）

```json
{
  "base_url": "https://kanban.example.com",
  "mcp_url": "https://kanban.example.com/mcp",
  "install_url": "https://kanban.example.com/install.md",
  "skill_url": "https://kanban.example.com/skills/slate-kanban/SKILL.md",
  "token_env": "SLATE_TOKEN",
  "hosts": [
    {
      "id": "claude-code",
      "name": "Claude Code",
      "mcp":   [{"title": "添加 MCP 服务", "lang": "sh", "code": "claude mcp add ..."}],
      "skill": [{"title": "安装 skill",   "lang": "sh", "code": "mkdir -p ... && curl ..."}]
    }
  ]
}
```

令牌不在里面：所有命令都引用环境变量 `SLATE_TOKEN`。网页在用户刚创建完令牌时，自己在最前面加一行 `export SLATE_TOKEN='…'`，这样令牌明文只出现在浏览器和用户的终端里。

### 三种 host 的安装方式

| host | MCP | skill 目录 | 依据 |
|------|-----|-----------|------|
| Claude Code | `claude mcp add --transport http --scope user slate {地址}/mcp --header 'Authorization: Bearer ${SLATE_TOKEN}'`（单引号，让 shell 不展开，原样写进配置） | `~/.claude/skills/slate-kanban/` | Claude Code 官方 MCP 文档：`--scope user`；配置里的 `headers` 支持 `${VAR}` 引用环境变量，加载时才展开；文档列出的"读成空值"的凭据名里没有 `SLATE_TOKEN` |
| Cursor | `mcp.json` 里写 `url` 和 `headers`，令牌用 `${env:SLATE_TOKEN}` 引用；全局 `~/.cursor/mcp.json`，项目 `.cursor/mcp.json` | `~/.cursor/skills/slate-kanban/` | Cursor 官方文档的 MCP 一节 |
| Codex | `codex mcp add slate --url {地址}/mcp --bearer-token-env-var SLATE_TOKEN` | `~/.codex/skills/slate-kanban/` | 本机 `codex mcp add --help` |

三种 host 记录的都是环境变量的名字，令牌明文不会落进它们的配置文件，更不会进入会被提交的文件。
代价是：启动 host 的进程必须能读到 `SLATE_TOKEN`。说明页提示用户把 `export` 写进自己的 shell 配置（例如 `~/.zshrc`）；
如果 Cursor 桌面版没有继承终端的环境变量（表现是连接 401），就从装好变量的终端里启动它，或者在 `mcp.json` 里直接写令牌（这个文件不要提交）。

## 资源和 instructions

- 资源 `slate://skills/slate-kanban`：固定地址的资源，`text/markdown`，内容是 skill 全文。两个视图都注册，因为人也可能想把它装给别的 agent。
- agent 的 `instructions` 最后一段改成：完整流程在 `slate-kanban` skill 里，装了就先读；没装就读取资源 `slate://skills/slate-kanban`，经用户同意后存成 skills 目录里的 `slate-kanban/SKILL.md`。
  `instructions` 总长仍然不超过 1500 字（已有测试限制）。

## 防止 skill 和服务端脱节

skill 是写给模型的文字，没有编译器帮忙，所以用测试守住几条：

1. frontmatter 里 `name` 是 `slate-kanban`，`description` 非空。
2. agent 视图的每个工具，skill 里都出现过（新增工具没写进 skill 会失败）。
3. skill 里出现的、长得像工具名的词，必须是 agent 视图里真实存在的工具（拼错、改名或提到管理视图的工具会失败）。
4. skill 里出现的 `ErrXxx` 错误名，必须在测试里登记的真实错误里（改名会让测试编译失败，提醒同步 skill）。
5. `/skills/slate-kanban/SKILL.md`、MCP 资源、编译进二进制的内容三者完全一致。
6. 安装说明里的 JSON 片段必须是合法 JSON，每个 host 都有 MCP 和 skill 两组步骤，命令里的地址和请求一致，没有令牌明文。

## 限制与没做的事

- 只覆盖 Claude Code、Cursor、Codex 三种 host，其他 host（以及它们的配置格式）不在内；安装命令和目录依据的是本机已安装的命令行帮助和官方文档，host 改版后以其官方文档为准。
- 反向代理放在子路径下的部署暂不支持：地址里没有路径前缀。
- skill 只有中文一份：它是写给 agent 读的操作手册，不是界面文案。网页界面的五种语言是另一件事。
- skill 不会自动更新：服务升级后，用户重新执行安装说明里的下载命令即可。
- 并发上限、20 分钟续租、worktree 的命名是 skill 里的建议，不是服务端的强制规则；服务端强制的只有权限、租约和列的约束。
- 没有 MCP Prompts：host 对它的支持参差不齐，流程说明放在 skill 和 `instructions` 里。

## 代码对照

| 步骤 | 函数 | 文件 |
|------|------|------|
| skill 全文（编译时内嵌） | SlateKanban | skills/embed.go、skills/slate-kanban/SKILL.md |
| 生成结构化说明 | buildGuide | internal/web/api/connectapi/guide.go |
| 渲染 Markdown 安装说明 | renderMarkdown | internal/web/api/connectapi/markdown.go |
| 从请求算服务地址并校验 | baseURL | internal/web/api/connectapi/base.go |
| 路由与处理函数 | Register、RegisterPublic | internal/web/api/connectapi/api.go |
| MCP 资源 | registerResources、readSkill | internal/web/mcpapi/resource.go |
| 连接时的提示 | agentInstructions | internal/web/mcpapi/instructions.go |
| 装配 | NewConnect、setupRouter | internal/web/api/provider.go、internal/web/api/api.go |
