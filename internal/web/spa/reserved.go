package spa

import "strings"

// apiLikePath 判断路径是否属于 API 或后端专有路由，这类未匹配路径必须返回 JSON 404，不能回退到 index.html。
func apiLikePath(path string) bool {
	switch {
	case path == "/health", path == "/mcp", path == installPath:
		return true
	case strings.HasPrefix(path, "/api/"), strings.HasPrefix(path, "/skills/"), strings.HasPrefix(path, "/app/"):
		return true
	default:
		return false
	}
}

const installPath = "/install.md"
