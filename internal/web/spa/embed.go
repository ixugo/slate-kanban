//go:build embed_web

package spa

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// ContentFS 在 embed_web 构建下返回嵌入的前端产物。
func ContentFS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return fs.FS(nil)
	}
	return sub
}
