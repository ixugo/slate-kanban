// Package updateapi 提供程序自动更新：网页打开时问服务端有没有新版本，点「立即更新」时由服务端从
// GitHub release 下载对应平台的包、校验后替换本地程序。所有构建（包括本机自己编译的）都会检查、
// 提示并能自动更新；新 release 里当前平台的包上传完才提示，容器里的程序只提示、引导去下载。
package updateapi

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/web"
)

const (
	// repo 是发布程序包的 GitHub 仓库。
	repo = "ixugo/slate-kanban"
	// defaultWebBase 是 GitHub 网站的地址：版本号、说明和安装包都从网页地址取，不用有匿名额度限制的 API。
	defaultWebBase = "https://github.com"
	// maxNotesRunes 是返回给网页的更新说明长度上限（按字符数），免得一份超长的说明撑爆弹窗。
	maxNotesRunes = 4000
	// checkTimeout 是向 GitHub 查询最新版本的超时时间；GitHub 不通时不让网页等太久。
	checkTimeout = 10 * time.Second
	// downloadTimeout 是下载一个包的总超时时间。
	downloadTimeout = 5 * time.Minute
)

// Options 是创建更新接口的配置；除了 Current，其余字段只有测试需要改。
type Options struct {
	// Current 是当前程序的版本号，例如 v1.2.3。
	Current string
	// Container 表示程序跑在容器里；容器里替换文件重启就丢，所以只提示不更新。
	Container bool
	// WebBase 是 GitHub 网站地址，空则用默认。
	WebBase string
	// DownloadPrefix 是安装包地址前缀，下载地址是「前缀 + tag + / + 文件名」，空则用 WebBase 下的 release 下载路径。
	DownloadPrefix string
	// Client 是发请求用的客户端，空则用默认客户端。
	Client *http.Client
	// ExePath 是要被替换的程序路径，空则取当前进程的可执行文件。
	ExePath string
	// GOOS 和 GOARCH 决定下载哪个平台的包，空则用当前平台。
	GOOS, GOARCH string
	// Restart 在程序替换成功后被调用，参数是新程序的路径；返回 true 表示已安排用新程序重启当前进程，
	// 返回 false 表示当前环境没法自动重启。空则不重启。
	Restart func(exe string) bool
}

// API 是自动更新接口。
type API struct {
	u *updater
}

// updater 保存更新需要的状态：配置、最新版本的缓存和已经替换完成的版本。
type updater struct {
	opt Options

	mu       sync.Mutex // 保护下面的缓存字段，同一时刻只放一个请求去问 GitHub
	cached   *release
	cachedAt time.Time
	failedAt time.Time
	applied  string // 已经替换成功的版本；程序还没重启，当前版本号不会变

	applying sync.Mutex // 同一时刻只允许一次下载替换
}

// New 创建更新接口，补齐配置的默认值。
func New(opt Options) API {
	if opt.WebBase == "" {
		opt.WebBase = defaultWebBase
	}
	if opt.DownloadPrefix == "" {
		opt.DownloadPrefix = opt.WebBase + "/" + repo + "/releases/download/"
	}
	if opt.Client == nil {
		opt.Client = &http.Client{}
	}
	if opt.GOOS == "" {
		opt.GOOS = runtime.GOOS
	}
	if opt.GOARCH == "" {
		opt.GOARCH = runtime.GOARCH
	}
	if !opt.Container {
		_, err := os.Stat("/.dockerenv")
		opt.Container = err == nil
	}
	return API{u: &updater{opt: opt}}
}

// Register 注册更新接口，和其他 REST 接口同组挂在 /api/v1 下，启用口令时同样需要认证。
func Register(r gin.IRouter, a API) {
	r.GET("/update", web.WrapH(a.check))
	r.POST("/update", web.WrapH(a.apply))
}

// check 返回当前版本、最新版本和能不能立即更新。查询失败只记日志、按「没有更新」返回，网页不用报错。
func (a API) check(c *gin.Context, _ *struct{}) (*Info, error) {
	return a.u.info(c.Request.Context()), nil
}

// apply 下载并替换程序，成功后返回新版本号；能自动重启时服务端会在响应发出后用新程序重启，否则需要用户手动重启。
func (a API) apply(c *gin.Context, _ *struct{}) (*Applied, error) {
	out, err := a.u.apply(c.Request.Context())
	if err != nil {
		slog.Warn("自动更新失败", "err", err)
		return nil, err
	}
	return out, nil
}

// Applied 是更新成功后的返回。
type Applied struct {
	Version string `json:"version"`
	// Restarting 为真表示服务端已安排自动重启，网页等新版本起来后刷新即可；为假要提示用户手动重启。
	Restarting bool `json:"restarting"`
}

// CheckState 说明本次检查走到了哪一步，方便网页区分「已是最新」和「GitHub 连不上」。
type CheckState string

const (
	// CheckStateOK 表示成功拿到 GitHub 上的 release 信息。
	CheckStateOK CheckState = "ok"
	// CheckStateUnreachable 表示查询 GitHub 失败（网络、限流等）。
	CheckStateUnreachable CheckState = "unreachable"
)

// info 汇总网页需要的信息；所有构建都会去查最新版本，当前平台的包上传完才算有更新。
func (u *updater) info(ctx context.Context) *Info {
	out := &Info{Current: u.opt.Current}
	rel, err := u.latest(ctx, false)
	if err != nil {
		slog.Warn("查询最新版本失败", "err", err)
		out.CheckState = CheckStateUnreachable
		return out
	}
	out.CheckState = CheckStateOK
	out.Latest = rel.Tag
	out.URL = rel.URL
	out.Notes = truncateRunes(rel.Body, maxNotesRunes)
	out.HasUpdate = u.newerThanRunning(rel.Tag) && u.packagesReady(rel)
	out.CanApply = out.HasUpdate && u.canApply(rel)
	return out
}

// Info 是网页用来决定要不要弹窗的信息。
type Info struct {
	Current    string     `json:"current"`
	Latest     string     `json:"latest"`
	Notes      string     `json:"notes"`
	URL        string     `json:"url"`
	HasUpdate  bool       `json:"has_update"`
	CanApply   bool       `json:"can_apply"`
	CheckState CheckState `json:"check_state"`
}

// newerThanRunning 判断 tag 是否比正在运行的版本新；已经替换成功的版本不再算「有更新」。
func (u *updater) newerThanRunning(tag string) bool {
	u.mu.Lock()
	applied := u.applied
	u.mu.Unlock()
	if applied != "" && !newer(tag, applied) {
		return false
	}
	return newer(tag, u.opt.Current)
}

// truncateRunes 把文本截到最多 limit 个字符，超出的部分用省略号代替。
func truncateRunes(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}
