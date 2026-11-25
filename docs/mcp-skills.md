# MCP Skills 与自定义 Skill

## 版本与兼容范围

2026-11-25 核对：项目使用的 [Go MCP SDK v1.8.0](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0) 仍是最新稳定版；[SEP-2640](https://modelcontextprotocol.io/extensions/skills/overview) 原生 Go 支持仍在 [PR #1238](https://github.com/modelcontextprotocol/go-sdk/pull/1238) 中。因此保留 SDK 和现有有状态 Streamable HTTP，通过其扩展能力、自定义方法和资源模板兼容实现。没有引入 fork 或切换到 2026-07-28 无状态协议。

- 初始化声明 `capabilities.extensions["io.modelcontextprotocol/skills"] = {}`，同时声明 resources 能力。不宣称支持可选的 `resources/directory/read`。
- `skills/list` 提供名称、简介、资源 URI、字节数与 SHA-256 摘要；支持游标分页。`skills/get` 根据规范 URI 返回当前单项的相同结构。目录不含正文；`resources/read` 读取 `skill://<name>/SKILL.md`。未知或已删除的 URI 返回 Invalid params（-32602）。旧有状态协议使用同一时期的结果信封。
- 明确声明 Skills 支持的宿主才收到新的精简初始化和 `get_board.watch_prompt`。根据场景读下面的 Skill。动态列指令、工程师名册、项目说明和必要调度约束继续下发。
- 旧客户端仍可读取原有 `slate://skills/slate-kanban` 完整说明、`/skills/slate-kanban/SKILL.md` 公开下载及原有守候提示。其工具列表、待办/审查流程、会话与订阅保持兼容。普通资源客户端可读 `slate://skills/catalog` 获取新目录，再按 URI 读取正文。

## 内置 Skill

| Skill | 加载时机 |
| --- | --- |
| `slate-kanban` | 调度入口、项目定位、必要约束 |
| `slate-watch` | 长期守候、监听或断线恢复 |
| `slate-development` | 研发派发、方案确认、交接或返工 |
| `slate-review` | 审查、验收证据、本地合并（适用时） |
| `slate-card-writing` | 卡片及评论写作、列表查询语法、截图 |
| `slate-management` | 用户明确要求管理时 |

原完整手册保留旧客户端使用。专题 Skill 复用其中的对应章节，并将原来的章节引用调整为规范 Skill URI；写作规则由新旧入口共用。

## 自定义 Skill

打开网页或桌面端「全局设置 → Skill」，创建、编辑、查看、删除。内置项只读。新建时填写名称、简介及 Markdown 正文；服务生成 `SKILL.md` 的 YAML frontmatter。名称使用 1–64 位小写字母或数字（可用单个连字符分隔），不能与内置或自定义 Skill 重名。简介为不超过 1024 字符的非空单行文本；正文是最多 256 KiB 的非空 UTF-8 Markdown（受现有 HTTP 1 MiB 请求体限制）。首版不支持额外 frontmatter 字段或辅助文件。无需另外创建项目。

服务持久化完整 Markdown 与 manifest，并在同一次数据库写入中更新。目录查询不读取数据库中的正文。更新后已连接的 MCP 会话重新调用 `skills/list` 或 `skills/get` 即可发现新摘要，无需重启。改名会改变 URI，删除会使 URI 失效。宿主应按 SEP 校验内容、处理缓存与用户批准；服务不决定宿主是否实际将 Skill 注入模型上下文。自定义内容及目录使用既有 REST/MCP 口令保护，不走旧的公开下载接口。

REST 契约：[skill.go.yaml](api/skill.go.yaml)。

## 复现：测量而不是估算

使用原有测量脚本，比较同一个隔离合成任务的初始化、工具列表、看板及卡片读取、旧完整 Skill、新目录和实际按需读取的 Skill：

```sh
go build -ldflags '-X main.buildVersion=slate-161-measure' -o /tmp/slate-skills .
uv run --no-project --with 'tiktoken==0.12.0' python docs/measure_mcp_tokens.py --binary /tmp/slate-skills --skills
```

使用同一脚本和参数对基线二进制重跑；没有 Skills 能力的基线会保留旧读取路径。可加 `--codex` 运行原有的宿主 Responses 捕获（仅统计实际外发 Slate 工具命名空间及工具输出，不代表生产计费或实际 Skill 激活）。

不允许监听端口时，使用实际 Streamable HTTP Handler 和 SQLite 生成同一合成任务的回复快照：

```sh
SLATE_SKILL_WIRE_SNAPSHOT=/tmp/slate-skills-wire.json go test ./internal/web/mcpapi -run '^TestSkillsWireMeasurement$' -count=1
uv run --no-project --with 'tiktoken==0.12.0' python docs/measure_mcp_tokens.py --snapshot /tmp/slate-skills-wire.json
# tokenizer 词表不可用时，明确统计 UTF-8 字节而非 token：
python3 docs/measure_mcp_tokens.py --snapshot /tmp/slate-skills-wire.json --bytes
```

本次隔离 Handler 测量（macOS arm64，Go 1.27.2，MCP 2025-06-18，单位为 UTF-8 字节），两种会话使用同一服务和任务。旧客户端入口保持原完整文案，因此这是一项新旧入口对照，不是对已部署主机的实测。

| 项目 | 旧兼容入口 | Skills 会话 |
| --- | ---: | ---: |
| 初始化说明正文 | 1,098 | 624 |
| 初始化 JSON-RPC 回复 | 1,437 | 952 |
| `tools/list` 回复 | 16,917 | 16,917 |
| `get_board` 回复 | 31,119 | 20,517 |
| `watch_prompt` 正文 | 6,677 | 1,454 |
| `get_card` 回复 | 7,506 | 7,506 |
| `skills/list` 回复（六个元数据项） | — | 2,458 |
| `skills/get` 单项元数据回复 | — | 501 |
| 完整旧 Skill / 新核心 Skill 正文 | 30,478 | 5,459 |
| 研发 Skill 正文（显式加载） | — | 4,800 |
| 写作 Skill 正文（显式加载） | — | 1,883 |

工具列表原样保留，核心入口与守候提示体积显著降低。目录、初始化及工具列表不含专题正文。正文仅在客户端显式读取时返回。测量中的词表下载及环回监听受本次沙箱限制，因此没有实测 o200k_base token、独立旧二进制网络回复、真实 AI 宿主注入或生产计费，不能把字节降幅等同于 token 降幅。
