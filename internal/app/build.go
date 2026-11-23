package app

import (
	"expvar"
	"time"

	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/pkg/orm"
)

// BuildInfo 是服务端和桌面端共享的版本、指标及迁移信息。
type BuildInfo struct {
	Version string
	Branch  string
	Commit  string
	Time    string
}

// Register 初始化当前进程的构建信息，应在装配 HTTP 服务之前调用一次。
func (info BuildInfo) Register() {
	expvar.NewString("version").Set(info.Version)
	expvar.NewString("git_branch").Set(info.Branch)
	expvar.NewString("git_hash").Set(info.Commit)
	expvar.NewString("build_time").Set(info.Time)
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Format(time.DateTime)
	}))

	// 本机每次启动都补齐新增的表结构，即使程序版本号相同。
	orm.SetEnabledAutoMigrate(true)
	versionapi.DBVersion = info.Version
	versionapi.DBRemark = info.Branch + "_" + info.Commit
}
