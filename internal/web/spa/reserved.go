package spa

import "strings"

// apiLikePath 判断路径是否属于 API 或后端专有路由，这类未匹配路径必须返回 JSON 404，不能回退到 index.html。
func apiLikePath(path string) bool {
	switch {
	case path == "/health", path == "/mcp", path == installPath:
		return true
	case strings.HasPrefix(path, "/api/"), strings.HasPrefix(path, "/skills/"), strings.HasPrefix(path, "/app/"), strings.HasPrefix(path, "/mcp/"):
		return true
	default:
		return false
	}
}

const installPath = "/install.md"

// noSpaFallback 判断路径在文件缺失时是否禁止回退到 index.html（SW、manifest、图标缺失时不能返回 HTML）。
// /.well-known/ 也一样：MCP 客户端收到 401 后会到这里找授权服务器的元数据，
// 返回 HTML 会让它们按 JSON 解析失败（Claude Code 会报 SyntaxError），必须是 404。
func noSpaFallback(urlPath string) bool {
	switch {
	case urlPath == "/sw.js", urlPath == "/manifest.webmanifest":
		return true
	case strings.HasPrefix(urlPath, "/icons/"), strings.HasPrefix(urlPath, "/.well-known/"):
		return true
	default:
		return false
	}
}
