# Slate

**面向 harness engineering 与 loop engineering 的看板。** 你来设计看板：列、提示词，以及决定并发数的工程师。编码 Agent 在上面跑循环：领取卡片、在 git worktree 里干活、交付审查、等待下一张。

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [한국어](README.ko.md) · [日本語](README.ja.md)

## 安装

1. 打开[最新 release](https://github.com/ixugo/slate-kanban/releases/latest)，下载对应系统的包：

| 系统 | 包 |
|---|---|
| macOS（Apple 芯片） | `slate_<version>_darwin_arm64.tar.gz` |
| Linux（x86-64） | `slate_<version>_linux_amd64.tar.gz` |
| Linux（ARM64） | `slate_<version>_linux_arm64.tar.gz` |
| Windows（x86-64） | `slate_<version>_windows_amd64.zip` |

2. 解压并运行。Windows 解压后运行 `slate.exe`。

```sh
tar -xzf slate_*.tar.gz
./slate
```

3. 打开 [http://127.0.0.1:7070](http://127.0.0.1:7070)。

- macOS 提示无法打开时，执行一次 `xattr -d com.apple.quarantine slate`。
- 想让 Slate 在后台运行并随登录自启（macOS 和 Linux），执行 `./slate -d`；停止并删除服务用 `./slate -stop`。
- 有新 release 时，网页会弹出提示。点击 **立即更新**，然后重启 Slate。

## 开始使用

1. 点击 **新建项目**，保留 **研发流程** 模板：**收件箱 → 待办 → 进行中 → 审查 → 完成**。收件箱归你；**待办** 和 **进行中** 由 Agent 管理。
2. 打开 **Agent** 页面，把一句话提示词复制给 Claude Code、Cursor 或 Codex，它会安装 MCP 和 Slate skill。
3. 添加卡片，写清要做什么、怎么验收。准备好后把卡片放进 **待办**。
4. 让 Agent 长期运行：在 **Agent** 页面找到 **长期挂着**，选好项目，把提示词粘进在项目目录里启动的 Agent 会话（放在 `screen` 或 `tmux` 里，关掉终端也不会断）。Agent 会循环：等待更新、从 **待办** 领取、移到 **进行中**、交付到 **审查**。
5. 在 **审查** 里检查交付内容。通过就移到 **完成**，不通过就留言并退回 **进行中**。

## 设计 harness

- **列提示词**（**设置 → 列**）告诉 Agent 每一列里能做什么。
- **工程师**（**设置 → 工程师**）是并发名额：一位工程师同一时间只做一张卡。可以招募全栈、前端、后端、UI 和测试工程师。
- **自动化规则**（**设置 → 自动化**）在事件发生时移动卡片、设置字段。

## 数据与访问

- 数据保存在 `~/.slate-kanban/`。备份前先停止 Slate，把 `data.db` 和 `resource/` 一起备份。
- Slate 监听 `0.0.0.0:7070`。没设口令时，能访问它的人都能读写全部看板。需要共享访问时，设置 `Server.HTTP.Passcode`，放在 HTTPS 反向代理后面，并把域名加进 `Server.HTTP.AllowedHosts`。参见[配置样例](configs/config.example.toml)。

## 许可

见 [LICENSE](LICENSE)。
