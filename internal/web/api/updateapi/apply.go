package updateapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ixugo/goddd/pkg/reason"
)

const (
	// maxArchiveBytes 是下载的包的大小上限；现在每个包十几 MB，留足余量即可，防止被喂一个无限大的响应。
	maxArchiveBytes = 200 << 20
	// maxBinaryBytes 是解出来的程序的大小上限，同理。
	maxBinaryBytes = 400 << 20
	// maxChecksumBytes 是校验文件的大小上限。
	maxChecksumBytes = 1 << 20
	// checksumsName 是 release 里放各个包 sha256 的文件名。
	checksumsName = "checksums.txt"
)

// archiveName 返回当前平台的包名，规则和发布工作流一致：slate_<tag>_<系统>_<架构>，windows 为 .zip，其余为 .tar.gz。
func (u *updater) archiveName(tag string) string {
	ext := "tar.gz"
	if u.opt.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("slate_%s_%s_%s.%s", tag, u.opt.GOOS, u.opt.GOARCH, ext)
}

// binaryName 是包里可执行文件的名字。
func (u *updater) binaryName() string {
	if u.opt.GOOS == "windows" {
		return "slate.exe"
	}
	return "slate"
}

// assetURL 按文件名在 release 里找已确认存在的下载地址，没有就返回空串。
func (u *updater) assetURL(rel *release, name string) string {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

// canApply 判断能不能自动更新：不在容器里，并且 release 里有当前平台的包和校验文件。
func (u *updater) canApply(rel *release) bool {
	return !u.opt.Container &&
		u.assetURL(rel, u.archiveName(rel.Tag)) != "" && u.assetURL(rel, checksumsName) != ""
}

// apply 取最新 release，下载、校验并替换程序，返回新版本号。每一步失败都不会改动现有程序。
func (u *updater) apply(ctx context.Context) (string, error) {
	if !u.applying.TryLock() {
		return "", reason.ErrConflict.WithMsg("正在更新，请稍候")
	}
	defer u.applying.Unlock()
	rel, err := u.latest(ctx, true)
	if err != nil {
		return "", reason.ErrNetworkError.WithMsg("无法从 GitHub 获取最新版本：" + err.Error())
	}
	if !u.newerThanRunning(rel.Tag) {
		return "", reason.ErrBadRequest.WithMsg("已经是最新版本")
	}
	if !u.settled(rel) {
		return "", reason.ErrBadRequest.WithMsg("新版本刚发布，安装包可能还没传完，请 5 分钟后再试")
	}
	if !u.canApply(rel) {
		return "", reason.ErrBadRequest.WithMsg("当前环境不支持自动更新，请到 release 页面手动下载")
	}
	if err := u.install(ctx, rel); err != nil {
		return "", reason.ErrServer.WithMsg("更新失败：" + err.Error())
	}
	u.mu.Lock()
	u.applied = rel.Tag
	u.mu.Unlock()
	return rel.Tag, nil
}

// install 下载包和校验文件，比对 sha256，解出程序并替换当前程序；临时文件在任何结果下都会清理。
func (u *updater) install(ctx context.Context, rel *release) error {
	exe, err := u.exePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	name := u.archiveName(rel.Tag)
	archive, sum, err := u.download(ctx, u.assetURL(rel, name), dir)
	if err != nil {
		return err
	}
	defer os.Remove(archive)
	sums, err := u.fetchSmall(ctx, u.assetURL(rel, checksumsName))
	if err != nil {
		return err
	}
	if want, ok := checksumFor(sums, name); !ok || !strings.EqualFold(want, sum) {
		return errors.New("包的 sha256 和 checksums.txt 对不上，已放弃更新")
	}
	fresh, err := extractBinary(archive, u.binaryName(), dir)
	if err != nil {
		return err
	}
	if err := replaceExe(exe, fresh, u.opt.GOOS); err != nil {
		return errors.Join(err, os.Remove(fresh))
	}
	return nil
}

// exePath 返回要替换的程序路径，跟着符号链接走到真实文件。
func (u *updater) exePath() (string, error) {
	p := u.opt.ExePath
	if p == "" {
		var err error
		if p, err = os.Executable(); err != nil {
			return "", fmt.Errorf("找不到当前程序的路径: %w", err)
		}
	}
	return filepath.EvalSymlinks(p)
}

// download 把包下载到程序所在目录的临时文件（同一个文件系统，后面才能原子改名），返回路径和 sha256。
func (u *updater) download(ctx context.Context, url, dir string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	res, err := u.get(ctx, url)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	f, err := os.CreateTemp(dir, ".slate-download-*")
	if err != nil {
		return "", "", fmt.Errorf("没有权限写入程序所在目录: %w", err)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, maxArchiveBytes+1))
	if err = errors.Join(err, f.Close()); err == nil && n > maxArchiveBytes {
		err = errors.New("下载的包超出大小上限")
	}
	if err != nil {
		return "", "", errors.Join(err, os.Remove(f.Name()))
	}
	return f.Name(), hex.EncodeToString(h.Sum(nil)), nil
}

// get 发 GET 请求，状态码不是 200 当作失败。
func (u *updater) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	res, err := u.opt.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("下载返回 %d", res.StatusCode)
	}
	return res, nil
}

// fetchSmall 下载一个小文件的全部内容。
func (u *updater) fetchSmall(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	res, err := u.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return io.ReadAll(io.LimitReader(res.Body, maxChecksumBytes))
}

// checksumFor 在 sha256sum 格式的文本里（每行「哈希 文件名」）找某个文件的哈希。
func checksumFor(sums []byte, name string) (string, bool) {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return fields[0], true
		}
	}
	return "", false
}

// extractBinary 从包里取出名为 bin 的文件，写到 dir 下的临时文件并设为可执行，返回路径。
// 只按名字精确匹配一个条目，不会按包里的路径写文件，所以不存在路径穿越。
func extractBinary(archive, bin, dir string) (string, error) {
	var open func() (io.ReadCloser, error)
	if isZip(archive) {
		open = func() (io.ReadCloser, error) { return openZipEntry(archive, bin) }
	} else {
		open = func() (io.ReadCloser, error) { return openTarEntry(archive, bin) }
	}
	src, err := open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	f, err := os.CreateTemp(dir, ".slate-new-*")
	if err != nil {
		return "", err
	}
	n, err := io.Copy(f, io.LimitReader(src, maxBinaryBytes+1))
	if err = errors.Join(err, f.Close(), os.Chmod(f.Name(), 0o755)); err == nil && n > maxBinaryBytes {
		err = errors.New("包里的程序超出大小上限")
	}
	if err != nil {
		return "", errors.Join(err, os.Remove(f.Name()))
	}
	return f.Name(), nil
}
