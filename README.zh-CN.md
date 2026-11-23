# Slate

**面向 harness engineering 与 loop engineering 的看板。** 你来设计看板：列、提示词，以及决定并发数的工程师。编码 Agent 在上面跑循环：领取任务、在 git worktree 里干活、交付审查、等待下一个。

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [日本語](README.ja.md)

## 安装

1. 打开[最新 release](https://github.com/ixugo/slate-kanban/releases/latest)，下载对应系统的包：

| 系统 | 包 |
|---|---|
| macOS（Apple 芯片） | `slate_<version>_darwin_arm64.tar.gz` |
| Linux（x86-64） | `slate_<version>_linux_amd64.tar.gz` |
| Linux（ARM64） | `slate_<version>_linux_arm64.tar.gz` |
| Windows（x86-64） | `slate_<version>_windows_amd64.zip` |

2. 解压到一个新目录，进入该目录运行。包里有程序和启动、停止脚本（`start.sh`、`stop.sh`；Windows 是 `start.bat`、`stop.bat`）。Windows 解压到一个目录后运行其中的 `slate.exe`。

```sh
mkdir slate && tar -xzf slate_*.tar.gz -C slate
cd slate
./slate
```

3. 打开 [http://127.0.0.1:7070](http://127.0.0.1:7070)。

- macOS 提示无法打开时，执行一次 `xattr -d com.apple.quarantine slate`。
- 想让 Slate 在后台运行并随登录自启（macOS 和 Linux），执行 `./slate -d`；停止并删除服务用 `./slate -stop`。
- 有新 release 时，网页会弹出提示。点击 **立即更新**，Slate 会自动重启，页面随后自动刷新。（以 Windows 服务运行时需要手动重启服务。）

## 开始使用

1. 点击 **新建项目**，保留 **研发流程** 模板：**收件箱 → 待办 → 进行中 → 审查 → 完成**。收件箱归你；**待办** 和 **进行中** 由 Agent 管理。
2. 打开 **Agent** 页面，把一句话提示词复制给 Claude Code、Cursor 或 Codex，它会安装 MCP 和 Slate skill。
3. 添加任务，写清要做什么、怎么验收。准备好后把任务放进 **待办**。
4. 让 Agent 长期运行：在 **Agent** 页面找到 **长期挂着**，选好项目，把提示词粘进在项目目录里启动的 Agent 会话（放在 `screen` 或 `tmux` 里，关掉终端也不会断）。Agent 会循环：等待更新、从 **待办** 领取、移到 **进行中**、交付到 **审查**。
5. 在 **审查** 里检查交付内容。通过就移到 **完成**，不通过就留言并退回 **进行中**。

## 设计 harness

- **列提示词**（**设置 → 列**）告诉 Agent 每一列里能做什么。
- **工程师**（**设置 → 工程师**）是并发名额：一位工程师同一时间只做一个任务。可以招募全栈、前端、后端、UI 和测试工程师。
- **自动化规则**（**设置 → 自动化**）在事件发生时移动任务、设置字段。

## MCP token 测量

测量使用隔离的 SQLite 工作区、固定的中英双语样例、MCP 2025-06-18 和 `o200k_base`（tiktoken 0.12.0）。样例包括一个空看板和一个有四张任务卡的看板，其中一张长任务卡有 1,770 字符的描述、五条评论和两个依赖。基线版本为 `a8a7a03`，复测版本为 `e5f8c54`。

| 工具发现 | 基线默认视图 | 改后默认视图 | 基线管理视图 | 改后旧版管理视图 | 改后披露管理工具 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 可见工具数 | 19 | 20 | 25 | 22 | 28 |
| 服务端指令 | 817 | 153 | 241 | 40 | 153 |
| MCP `initialize` 回复 | 931 | 233 | 332 | 117 | 233 |
| MCP `tools/list` 回复 | 5,517 | 4,094 | 7,139 | 5,922 | 6,493 |
| Codex 0.170.0 发往 Responses 的 Slate 命名空间 | 6,031 | 3,898 | 6,943 | 5,566 | 客户端保留默认的 3,898 |
| Codex 发往 Responses 的 `whoami` 结果对象 | 101 | 94 | 104 | 94 | — |

`enable_management` 在默认视图中增加了一个入口。撤下三个旧工具后，各视图去重后的工具数从 30 降到 28。默认工具列表的 token 减少了 25.8%，Codex 发出的 Slate 命名空间减少了 35.4%。Codex 在命名空间中只包含一份服务端指令。这些数字来自对捕获报文的 tokenizer 计数，已登录宿主的尝试未取得供应商用量。

| 七次调用的调度样例 | 请求 token | 回复 token |
| --- | ---: | ---: |
| 基线 | 277 | 13,737 |
| 改后，保留原参数 | 277 | 13,741 |
| 改后，选择精简看板读取 | 285 | 8,044 |

沿用原参数时的微小差异来自样例的时间戳。精简流程使用 `list_boards(summary=true)` 和 `get_board(include_watch_prompt=false)`。原有调用仍返回完整看板数据。改前和改后的空等回复都是 93 token，评论唤醒等待的回复都是 261 token。事件监听器可在模型回合之外等待。

[复现命令、逐工具计数、调用明细与宿主验证](docs/mcp-token-baseline.md#reproduce).

### 通过 MCP 管理 Slate

用标准配置连接一次 Slate MCP。需要更改看板设置、列或自动化时，明确告诉 Agent。Agent 通过 `enable_management` 在当前连接披露管理工具。Codex 0.170.0 在同一线程的下一回合仍沿用默认工具列表。工具列表固定的宿主可以通过 `include_tools: true` 获取目录，再用 `tool` 指定管理工具名、用 `arguments` 传参数，调用一次 `enable_management`。隔离的 Codex 测试通过此入口实际创建了列。

现有工作连接和旧版管理连接仍可使用。撤下的三个 MCP 工具可通过 REST 实现相同操作。用 `GET /api/v1/boards/{id}/events` 读活动，用 `DELETE /api/v1/cards/{id}` 删任务，用 `DELETE /api/v1/comments/{id}` 删评论。任务的规划字段可以在旧版管理连接、Slate 网页或 REST 中管理。新配置只需保留一条标准连接。

### 通过 MCP 搜索任务

向 `list_cards` 的 `query` 传入搜索词和筛选条件，语法与网页搜索相同。关键词匹配标题、描述、任务编号和标签。把示例中的 `board_id` 换成要搜索的看板编号。

```json
{"board_id":1,"query":"\"login timeout\" is:open","limit":10}
```

结果包含任务摘要、命中总数 `total` 和截断标记 `truncated`。用 `get_card` 读取命中任务的完整内容和评论。

更新搜索说明后，默认的 20 项工具对应的 `tools/list` 回复由 4,094 降至 4,092 token。[工具发现复测与复现方法](docs/mcp-token-baseline.md#discovery-retest-for-task-search)。

## 数据与访问

- 数据保存在 `~/.slate-kanban/`。备份前先停止 Slate，把 `data.db` 和 `resource/` 一起备份。
- Slate 监听 `0.0.0.0:7070`。没设口令时，能访问它的人都能读写全部看板。需要共享访问时，设置 `Server.HTTP.Passcode`，放在 HTTPS 反向代理后面。参见[配置样例](configs/config.example.toml)。

## 许可

见 [LICENSE](LICENSE)。
