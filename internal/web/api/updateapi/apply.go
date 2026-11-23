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

// archiveName 返回与发布工作流一致的当前平台包名。
func (u *updater) archiveName(tag string) string {
	ext := "tar.gz"
	switch {
	case u.opt.GOOS == "windows":
		ext = "zip"
	case u.opt.Desktop && u.opt.GOOS == "darwin":
		ext = "dmg"
	}
	return fmt.Sprintf("%s_%s_%s_%s.%s", u.packageName(), tag, u.opt.GOOS, u.opt.GOARCH, ext)
}

func (u *updater) packageName() string {
	if u.opt.Desktop {
		return "slate_desktop"
	}
	return "slate"
}

func (u *updater) checksumAsset() string {
	if u.opt.Desktop {
		return "desktop-checksums.txt"
	}
	return checksumsName
}

// binaryName 是包里可执行文件的名字。
func (u *updater) binaryName() string {
	name := u.packageName()
	if u.opt.GOOS == "windows" {
		name += ".exe"
	}
	return name
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
	if u.opt.Desktop && u.opt.GOOS == "darwin" && u.opt.OpenInstaller == nil {
		return false
	}
	return !u.opt.Container && u.packagesReady(rel)
}

// apply 取最新 release，下载、校验并替换程序，替换成功后请求重启。每一步失败都不会改动现有程序，也不会请求重启。
func (u *updater) apply(ctx context.Context) (*Applied, error) {
	if !u.applying.TryLock() {
		return nil, reason.ErrConflict.WithMsg("正在更新，请稍候")
	}
	defer u.applying.Unlock()
	rel, err := u.latest(ctx, true)
	if err != nil {
		return nil, reason.ErrNetworkError.WithMsg("无法从 GitHub 获取最新版本：" + err.Error())
	}
	if !u.newerThanRunning(rel.Tag) {
		return nil, reason.ErrBadRequest.WithMsg("已经是最新版本")
	}
	if !u.packagesReady(rel) {
		return nil, reason.ErrBadRequest.WithMsg("新版本的安装包还没上传完，请稍后再试")
	}
	if !u.canApply(rel) {
		return nil, reason.ErrBadRequest.WithMsg("当前环境不支持自动更新，请到 release 页面手动下载")
	}
	if u.opt.Desktop && u.opt.GOOS == "darwin" {
		if err := u.openDiskImage(ctx, rel); err != nil {
			return nil, reason.ErrServer.WithMsg("打开安装包失败：" + err.Error())
		}
		return &Applied{Version: rel.Tag, InstallerOpened: true}, nil
	}
	exe, err := u.install(ctx, rel)
	if err != nil {
		return nil, reason.ErrServer.WithMsg("更新失败：" + err.Error())
	}
	u.mu.Lock()
	u.applied = rel.Tag
	u.mu.Unlock()
	return &Applied{Version: rel.Tag, Restarting: u.opt.Restart != nil && u.opt.Restart(exe)}, nil
}

// install 下载包和校验文件，比对 sha256，解出程序并替换当前程序，返回被替换的程序路径；临时文件在任何结果下都会清理。
func (u *updater) install(ctx context.Context, rel *release) (string, error) {
	exe, err := u.exePath()
	if err != nil {
		return "", err
	}
	return exe, u.replaceWith(ctx, rel, exe)
}

// replaceWith 把 release 里当前平台的包下载、校验、解压后替换到 exe 位置。
func (u *updater) replaceWith(ctx context.Context, rel *release, exe string) error {
	dir := filepath.Dir(exe)
	archive, err := u.downloadVerified(ctx, rel, dir)
	if err != nil {
		return err
	}
	defer os.Remove(archive)
	fresh, err := extractBinary(archive, u.binaryName(), dir)
	if err != nil {
		return err
	}
	if err := replaceExe(exe, fresh, u.opt.GOOS); err != nil {
		return errors.Join(err, os.Remove(fresh))
	}
	return nil
}

// openDiskImage 将通过校验的更新包交给 macOS 打开，由用户在 Finder 安装新应用。
func (u *updater) openDiskImage(ctx context.Context, rel *release) error {
	dir := u.opt.DesktopUpdateDir
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	archive, err := u.downloadVerified(ctx, rel, dir)
	if err != nil {
		return err
	}
	defer os.Remove(archive)
	image := filepath.Join(dir, "Slate-Update.dmg")
	if err := os.Rename(archive, image); err != nil {
		return err
	}
	return u.opt.OpenInstaller(ctx, image)
}

func (u *updater) downloadVerified(ctx context.Context, rel *release, dir string) (string, error) {
	name := u.archiveName(rel.Tag)
	archive, sum, err := u.download(ctx, u.assetURL(rel, name), dir)
	if err != nil {
		return "", err
	}
	sums, err := u.fetchSmall(ctx, u.assetURL(rel, u.checksumAsset()))
	if err == nil {
		if want, ok := checksumFor(sums, name); !ok || !strings.EqualFold(want, sum) {
			err = fmt.Errorf("%s 的 sha256 与 %s 不一致", name, u.checksumAsset())
		}
	}
	if err != nil {
		return "", errors.Join(err, os.Remove(archive))
	}
	return archive, nil
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
