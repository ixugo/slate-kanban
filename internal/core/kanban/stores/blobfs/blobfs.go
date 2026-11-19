// Package blobfs 把图片等资源文件存在本地目录里，是 kanban.ResourceStorer 的文件系统实现。
package blobfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.ResourceStorer = Dir{}

const (
	// dirPerm 是资源目录的权限：只有当前用户能写，其他人只读。
	dirPerm = 0o755
	// filePerm 是资源文件的权限。
	filePerm = 0o644
	// tmpSuffix 是写入过程中临时文件的后缀，写完再改名，读的人不会看到写了一半的文件。
	tmpSuffix = ".tmp"
)

// Dir 是以某个目录为根的资源存储。
type Dir struct {
	root string
}

// New 创建资源存储，root 是资源根目录；目录在第一次写入时才创建。
func New(root string) Dir {
	return Dir{root: root}
}

// resolve 把相对路径转成根目录下的真实路径。空路径、绝对路径和越出根目录的路径（带 ..）都拒绝，
// 这样即使上层传进来的值有问题，也不会读写或删除资源目录之外的文件。
func (d Dir) resolve(rel string) (string, error) {
	if d.root == "" || rel == "" || filepath.IsAbs(rel) || strings.ContainsRune(rel, '\\') {
		return "", fmt.Errorf("非法的资源路径: %q", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("非法的资源路径: %q", rel)
	}
	return filepath.Join(d.root, clean), nil
}

// Save 先写临时文件再改名，保证同一路径上任何时刻读到的要么是旧文件要么是完整的新文件。
func (d Dir) Save(rel string, data []byte) error {
	full, err := d.resolve(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), dirPerm); err != nil {
		return fmt.Errorf("创建资源目录: %w", err)
	}
	tmp := full + tmpSuffix
	if err := os.WriteFile(tmp, data, filePerm); err != nil {
		return fmt.Errorf("写入资源文件: %w", err)
	}
	if err := os.Rename(tmp, full); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("落盘资源文件: %w", err)
	}
	return nil
}

// Read 读取文件内容，文件不存在时返回的错误满足 errors.Is(err, os.ErrNotExist)。
func (d Dir) Read(rel string) ([]byte, error) {
	full, err := d.resolve(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

// Remove 删除单个文件，文件本来就不在也算成功。
func (d Dir) Remove(rel string) error {
	full, err := d.resolve(rel)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// RemoveDir 删除根目录下的一个子目录。只允许一层目录名，防止误删根目录或更深层的内容。
func (d Dir) RemoveDir(dir string) error {
	if strings.ContainsAny(dir, `/\`) {
		return fmt.Errorf("非法的资源目录: %q", dir)
	}
	full, err := d.resolve(dir)
	if err != nil {
		return err
	}
	return os.RemoveAll(full)
}
