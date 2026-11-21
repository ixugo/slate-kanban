package updateapi

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

// zipMagic 是 zip 文件开头的两个字节，临时文件没有扩展名时靠它判断格式。
var zipMagic = []byte("PK")

// isEntry 判断包里的条目是不是要取的文件。新包把文件放在 slate_<tag>_<系统>_<架构>/ 目录下，
// 旧包平铺在根目录，两种都认，所以只比较最后一段名字。
func isEntry(entry, name string) bool {
	return path.Base(entry) == name
}

// isZip 判断文件是不是 zip：读开头两个字节。
func isZip(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, len(zipMagic))
	_, err = io.ReadFull(f, head)
	return err == nil && string(head) == string(zipMagic)
}

// readCloser 把读取端和一组要关闭的资源绑在一起，关闭时一起关。
type readCloser struct {
	io.Reader
	closers []io.Closer
}

// Close 依次关闭所有资源，返回合并后的错误。
func (r readCloser) Close() error {
	var err error
	for _, c := range r.closers {
		err = errors.Join(err, c.Close())
	}
	return err
}

// openZipEntry 打开 zip 里名为 name 的文件。
func openZipEntry(path, name string) (io.ReadCloser, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("打开 zip 包: %w", err)
	}
	for _, f := range zr.File {
		if isEntry(f.Name, name) && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return nil, errors.Join(err, zr.Close())
			}
			return readCloser{Reader: rc, closers: []io.Closer{rc, zr}}, nil
		}
	}
	return nil, errors.Join(fmt.Errorf("包里没有 %s", name), zr.Close())
}

// openTarEntry 打开 tar.gz 里名为 name 的普通文件。
func openTarEntry(path, name string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("打开 tar.gz 包: %w", err), f.Close())
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err != nil {
			return nil, errors.Join(fmt.Errorf("包里没有 %s", name), gz.Close(), f.Close())
		}
		if h.Typeflag == tar.TypeReg && isEntry(h.Name, name) {
			return readCloser{Reader: tr, closers: []io.Closer{gz, f}}, nil
		}
	}
}

// replaceExe 用新程序替换旧程序。类 Unix 直接改名覆盖，正在运行的旧程序不受影响；
// windows 不允许覆盖正在运行的 exe，所以先把旧的改名成 .old，再把新的放到原位，放不上去就把旧的改回来。
func replaceExe(exe, fresh, goos string) error {
	if goos != "windows" {
		return os.Rename(fresh, exe)
	}
	old := exe + ".old"
	if err := os.Remove(old); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("清理上一次留下的 %s: %w", filepath.Base(old), err)
	}
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(fresh, exe); err != nil {
		return errors.Join(err, os.Rename(old, exe))
	}
	return nil
}
