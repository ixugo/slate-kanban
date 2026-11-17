//go:build !embed_web

package spa

import (
	"io/fs"
	"os"
)

// ContentFS 在无 embed_web 标签时从磁盘读 dist，方便本地调试；目录不存在则返回空 FS。
func ContentFS() fs.FS {
	root := "internal/web/spa/dist"
	if _, err := os.Stat(root); err != nil {
		return fs.FS(nil)
	}
	sub, err := fs.Sub(os.DirFS(root), ".")
	if err != nil {
		return fs.FS(nil)
	}
	return sub
}
