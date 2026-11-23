package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"slate-kanban/internal/app"
	"slate-kanban/internal/conf"
	"slate-kanban/internal/svcmgr"
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

// -d 注册为系统服务并启动，-stop 停止并删除该服务（Windows 服务、Linux systemd、macOS launchd）
var (
	daemonMode = flag.Bool("d", false, "register as a system service and start it (Windows service / Linux systemd / macOS launchd)")
	stopMode   = flag.Bool("stop", false, "stop the service registered by -d and remove it")
)

func getBuildRelease() bool {
	v, _ := strconv.ParseBool(release)
	return v
}

func main() {
	flag.Parse()

	if *daemonMode || *stopMode {
		if *daemonMode && *stopMode {
			fmt.Fprintln(os.Stderr, "-d 和 -stop 不能同时使用")
			os.Exit(1)
		}
		svcmgr.RunDaemonFromMain(*stopMode, *configDir)
		return
	}

	if svcmgr.UnderServiceManager() {
		if err := svcmgr.RunAsService(runApp); err != nil {
			fmt.Fprintf(os.Stderr, "以服务方式运行失败: %v\n", err)
			os.Exit(1)
		}
		return
	}
	runApp(nil)
}

// runApp 加载配置并运行服务，阻塞到收到退出信号或 stop 被关闭；stop 为 nil 时只看信号。
func runApp(stop <-chan struct{}) {
	// 初始化配置
	fileDir, err := resolveConfigDir(*configDir)
	if err != nil {
		panic(err)
	}
	warnLegacyConfigDir(*configDir == "", filepath.Join(fileDir, "config.toml"))
	bc, closeConfig, err := app.OpenConfig(fileDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer closeConfig()

	bc.Runtime.Debug = !getBuildRelease()
	bc.Runtime.BuildVersion = buildVersion

	app.BuildInfo{Version: buildVersion, Branch: gitBranch, Commit: gitHash, Time: buildTime}.Register()

	// Run 返回时端口、数据库和日志都已释放；自动更新要求重启时，在这里换成新程序
	if exe := app.Run(&bc, stop); exe != "" {
		closeConfig()
		if err := app.Restart(exe); err != nil {
			fmt.Fprintf(os.Stderr, "更新后自动重启失败，请手动重启 Slate: %v\n", err)
			os.Exit(1)
		}
	}
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
