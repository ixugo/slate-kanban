package main

import (
	"expvar"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/system"
	"slate-kanban/internal/app"
	"slate-kanban/internal/conf"
	"slate-kanban/internal/svcinstall"
)

var (
	buildVersion = "0.0.1" // 构建版本号
	gitBranch    = "dev"   // git 分支
	gitHash      = "debug" // git 提交点哈希值
	release      string    // 发布模式 true/false
	buildTime    string    // 构建时间戳
)

// 自定义配置目录；不填就用 ~/.slate-kanban（配置文件和数据库都在里面）
var configDir = flag.String("conf", "", "config directory, default ~/.slate-kanban, eg: -conf /configs/")

func getBuildRelease() bool {
	v, _ := strconv.ParseBool(release)
	return v
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "service" {
		svcinstall.RunFromMain(os.Args[2:])
		return
	}

	flag.Parse()

	// 初始化配置
	var bc conf.Bootstrap
	fileDir, err := resolveConfigDir(*configDir)
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(fileDir, 0o755); err != nil {
		panic(err)
	}
	filePath := filepath.Join(fileDir, "config.toml")
	warnLegacyConfigDir(*configDir == "", filePath)
	configIsNotExistWrite(filePath, fileDir)
	if err := conf.SetupConfig(&bc, filePath); err != nil {
		panic(err)
	}
	// 口令配置不合规时在连数据库之前就退出，避免服务起来后 MCP 客户端莫名其妙地 401
	if err := conf.ValidatePasscode(&bc); err != nil {
		panic(err)
	}
	bc.Runtime.Debug = !getBuildRelease()
	bc.Runtime.BuildVersion = buildVersion
	bc.Runtime.ConfigDir = fileDir
	bc.Runtime.ConfigPath = filePath

	{
		expvar.NewString("version").Set(buildVersion)
		expvar.NewString("git_branch").Set(gitBranch)
		expvar.NewString("git_hash").Set(gitHash)
		expvar.NewString("build_time").Set(buildTime)
		expvar.Publish("timestamp", expvar.Func(func() any {
			return time.Now().Format(time.DateTime)
		}))
	}

	// 设置数据库版本号，用于驱动 orm 的 AutoMigrate
	// 本项目是开箱即用的单文件看板，每次启动都执行表迁移：已有的表只会补齐缺的列和索引，
	// 不会因为版本号没变（例如 go run 开发时）而漏掉新增的表结构
	orm.SetEnabledAutoMigrate(true)
	versionapi.DBVersion = buildVersion
	versionapi.DBRemark = gitBranch + "_" + gitHash

	app.Run(&bc)
}

// resolveConfigDir 决定配置目录：没指定就用 ~/.slate-kanban，指定了按启动目录转成绝对路径
// （system.Abs 以可执行文件目录为基准，go run 时会落进临时目录，所以不用它）。
func resolveConfigDir(flagValue string) (string, error) {
	if flagValue == "" {
		return conf.DefaultDir()
	}
	return filepath.Abs(flagValue)
}

// warnLegacyConfigDir 提醒旧版本的用户：默认配置目录从启动目录下的 ./configs 改成了 ~/.slate-kanban。
// 只在用默认目录、新目录还没有配置、而旧目录里有配置时提示一次，不自动搬数据，避免悄悄动用户的库。
func warnLegacyConfigDir(useDefault bool, newConfigPath string) {
	if !useDefault {
		return
	}
	if _, err := os.Stat(newConfigPath); err == nil {
		return
	}
	if _, err := os.Stat(filepath.Join("configs", "config.toml")); err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "提示：在当前目录发现旧位置的配置 ./configs，默认目录已改为 %s。\n要沿用旧数据，请先停掉服务，再把 configs/config.toml、data.db（含 -wal、-shm）复制过去，并把 config.toml 里的 Dsn、Log.Dir 改成新目录下 data.db、logs 的绝对路径。\n", filepath.Dir(newConfigPath))
}

// configIsNotExistWrite 配置文件不存在时，回写默认配置（数据库路径放在 dir 下）
func configIsNotExistWrite(path, dir string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := conf.WriteConfig(conf.DefaultConfig(dir), path); err != nil {
			system.ErrPrintf("WriteConfig", "err", err)
		}
	}
}
