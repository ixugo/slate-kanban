// Package skills 把随服务一起发布的 agent skill 编译进二进制。
// 服务直接把它交给 agent，所以 skill 和服务端的工具集永远是同一个版本。
package skills

import (
	_ "embed"
)

// slateKanban 是编译时内嵌的 slate-kanban skill 全文。
//
//go:embed slate-kanban/SKILL.md
var slateKanban string

// SlateKanban 返回 slate-kanban skill 的全文（SKILL.md）。
// 用函数而不是公开变量，是为了让调用方拿不到可以被改写的全局状态。
func SlateKanban() string {
	return slateKanban
}

// WatchURI 是 MCP 客户端获取 Python 监听脚本的资源地址。
const WatchURI = "slate://scripts/slate-watch.py"

// CodexWatchURI 保留给已在使用这个地址的客户端。
const CodexWatchURI = "slate://scripts/codex-watch.py"

//go:embed slate-kanban/watch.py
var watchScript string

// WatchScript 返回随当前服务发布的 Python 监听脚本。
func WatchScript() string {
	return watchScript
}
