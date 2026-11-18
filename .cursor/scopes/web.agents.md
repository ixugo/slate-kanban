# Agent 接入页

## 需求目标

用户按宿主（Cursor、Claude Code 等）复制 MCP 与 skill 安装命令，理解 `?name=`、补拉与 AI 列约定。

## 页面结构

- 路由 `/agents`
- 三步说明 + 一句话提示词（可复制）
- 按 `hosts` 分 Tab：worker MCP、折叠 manager MCP、skill 片段

## 决策

| 项 | 决定 |
|---|---|
| 数据 | `getConnectGuide` + `queryKeys.connect` |
| 标题 | 已知 `kind` 走 i18n，未知用服务端 `title` |
| 复制 | `clipboard.writeText`，失败选中文本 + toast |

## 验收清单

- [ ] 加载骨架、失败重试
- [ ] 各 kind 片段与复制
- [ ] 五语言说明文案