# Slate

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [한국어](README.ko.md) · [日本語](README.ja.md)

Slate 是一个以本机使用为主、由人和 AI Agent 共用的看板。在网页上写好任务，把准备就绪的卡片拖进 **进行中**，编码 Agent 就能通过 MCP 领取。Agent 完成后把卡片交到 **审查**，你可以在同一块看板上验收。

## 用 Docker 安装

先安装 Git、Docker 和 Docker Compose，再执行以下命令。

```sh
git clone https://github.com/ixugo/slate-kanban.git
cd slate-kanban
docker compose up -d --build
```

打开 [http://127.0.0.1:7070](http://127.0.0.1:7070)。Compose 会用检出的代码构建 Slate，并把 SQLite 数据库、附件和日志保存在 `slate-data` 卷里。升级时，更新同一目录里的代码后重新执行 Compose 命令。要停止 Slate 并保留数据，执行 `docker compose down`。

如需构建单文件程序或在本机开发，见 [源码构建与开发启动](docs/development.zh-CN.md)。

## 创建第一个项目

1. 点击 **新建项目**，填写名称，沿用默认的 **研发流程** 模板。看板会建好 **收件箱 → 待办 → 进行中 → 审查 → 完成**。也可以选择 **空白**，在 **设置 → 列** 中自己加列。
2. 点击 **新建卡片**，写清任务和验收标准。按需添加标签、优先级、截止日期或依赖。
3. 接入 Agent 后，把准备就绪的卡片拖进 **进行中** 交给 Agent。卡片进入 **审查** 后检查交接结果，通过就移到 **完成**，需要返工就留言并移回 **进行中**。

## 接入 AI Agent

打开 Slate 的 **Agent** 页面，把一句话提示复制给 Claude Code、Cursor 或 Codex。本机运行时，可以直接发送下面这句话。

```text
请阅读 http://127.0.0.1:7070/install.md ，按说明完成 Slate 接入。
```

也可以从 **Agent** 页面复制对应客户端的 MCP 配置和 skill 安装命令。每个 Agent 使用独立的 MCP `name`。日常领任务使用工作视图，有修改列或自动化规则的需要时再配置管理视图。

装好 MCP 和 Slate skill 后，新开一个 Agent 会话，让它调用 `whoami` 确认连接。接着让它处理你的项目。当它询问项目对应的仓库时，确认本地路径，方便它在对应的 Git worktree 里工作。

## 使用看板

- 在顶部栏切换界面语言，支持简体中文、繁体中文、英语、韩语和日语。
- 在看板和可排序的列表之间切换，用搜索框或筛选面板查找卡片。例如，`is:overdue -is:done` 会找出已逾期且未完成的卡片。
- 在 **设置 → 自动化** 添加规则。研发流程模板会记录卡片进入 **进行中** 和 **完成** 的时间，卡片离开 **完成** 时会清除完成时间。
- 在 **设置 → 导出** 下载项目的 JSON 或 Markdown。
- 通过 localhost 或 HTTPS 打开 Slate 时，可以用浏览器的安装功能把它放进独立窗口。离线时可以查看上次缓存的看板快照。

## 数据与访问

- 直接运行二进制时，数据默认保存在 `~/.slate-kanban/`。备份前停止 Slate，将 `data.db` 和 `resource/` 目录一起备份。使用 Docker 时，升级或重建容器请保留 `slate-data` 卷。
- Slate 默认监听 `0.0.0.0:7070`，允许通过本机和局域网地址访问。未设口令时，能访问 Slate 的人都可以读写全部看板。需要共享访问时，设置 `Server.HTTP.Passcode`，使用 HTTPS 反向代理，并在 `Server.HTTP.AllowedHosts` 填入对外域名。参见 [本机配置样例](configs/config.example.toml) 和 [Docker 配置样例](configs/config.docker.toml)。

## 许可

见 [LICENSE](LICENSE)。
