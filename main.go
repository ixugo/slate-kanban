package main

import (
	"expvar"
	"flag"
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

// 自定义配置目录
var configDir = flag.String("conf", "./configs", "config directory, eg: -conf /configs/")

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
	// 相对路径以启动目录为基准（system.Abs 以可执行文件目录为基准，go run 时会落进临时目录）
	fileDir, err := filepath.Abs(*configDir)
	if err != nil {
		panic(err)
	}
	_ = os.MkdirAll(fileDir, 0o755)
	filePath := filepath.Join(fileDir, "config.toml")
	configIsNotExistWrite(filePath)
	if err := conf.SetupConfig(&bc, filePath); err != nil {
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

// configIsNotExistWrite 配置文件不存在时，回写配置
func configIsNotExistWrite(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := conf.WriteConfig(conf.DefaultConfig(), path); err != nil {
			system.ErrPrintf("WriteConfig", "err", err)
		}
	}
}
