package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/ixugo/goddd/pkg/logger"
	"github.com/ixugo/goddd/pkg/server"
	"github.com/ixugo/goddd/pkg/system"

	"slate-kanban/internal/conf"
	"slate-kanban/internal/lifecycle"
)

// Run 装配依赖并启动 HTTP 服务，阻塞到收到退出信号或 stop 被关闭（stop 为 nil 时只看信号）；收到信号后先通知长连接结束，再等普通请求处理完并关闭。
// 相对路径（配置、数据库、日志）都以启动时的工作目录为基准，
// 以服务方式运行时请由服务管理器设置 WorkingDirectory。
// 返回值是要重启成的新程序路径：非空表示是自动更新请求的重启，调用方要在 Run 返回（资源已释放）后调用 Restart；空表示正常退出。
func Run(bc *conf.Bootstrap, stop <-chan struct{}) (restartExe string) {
	log, clean := SetupLog(bc)
	defer clean()

	requestRestart, restartCh := newRestartRequester()
	bc.Runtime.RequestRestart = requestRestart

	handler, cleanUp, err := WireApp(bc, log)
	if err != nil {
		slog.Error("程序构建失败", "err", err)
		panic(err)
	}
	defer cleanUp()

	// 关闭服务前先取消 closing，通知长连接（实时流、阻塞等待）自己结束；
	// 否则 Shutdown 要一直等到超时，还会留下一条错误日志
	closing, closeNow := context.WithCancel(context.Background())
	defer closeNow()
	handler = lifecycle.Wrap(handler, closing)

	addr := net.JoinHostPort(bc.Server.HTTP.Host, strconv.Itoa(bc.Server.HTTP.Port))
	svc := server.New(handler,
		server.Port(addr),
		server.ReadTimeout(bc.Server.HTTP.Timeout.Duration()),
		// 不设整体写超时：SSE 推送和 MCP 阻塞等待都是长连接，需要持续写入；
		// 普通接口的耗时由各自的 context 控制
		server.WriteTimeout(0),
	)
	go svc.Start()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("服务启动成功:", displayURL(bc))

	select {
	case s := <-interrupt:
		slog.Info(`<-shutdown`, "signal", s.String())
	case <-stop:
		slog.Info(`<-shutdown`, "signal", "service stop")
	case restartExe = <-restartCh:
		slog.Info(`<-shutdown`, "signal", "restart after update", "exe", restartExe)
	case err := <-svc.Notify():
		system.ErrPrintf("err: %s\n", err.Error())
		slog.Error(`<-shutdown by server.Notify`, "err", err)
	}
	closeNow()
	if err := svc.Shutdown(); err != nil {
		slog.Error(`server.Shutdown`, "err", err)
	}
	return restartExe
}

// SetupLog 初始化日志
func SetupLog(bc *conf.Bootstrap) (*slog.Logger, func()) {
	// 配置里写绝对路径就直接用（默认配置就是配置目录下的 logs），相对路径以启动目录为基准
	logDir := bc.Log.Dir
	if !filepath.IsAbs(logDir) {
		logDir = filepath.Join(system.Getwd(), logDir)
	}

	return logger.SetupSlog(logger.Config{
		FileConfig: logger.FileConfig{
			Dir:          logDir,                         // 日志地址
			MaxAge:       bc.Log.MaxAge,                  // 日志存储时间
			RotationTime: bc.Log.RotationTime.Duration(), // 循环时间
			MaxSize:      bc.Log.MaxSize,                 // 循环大小
			Compress:     bc.Log.Compress,                // 是否压缩日志
			MaxBackups:   bc.Log.MaxBackups,              // 保留的旧日志归档文件最大数量，超出的自动删除
		},
		Debug: bc.Runtime.Debug, // 服务级别Debug/Release
		Level: bc.Log.Level,     // 日志级别
	})
}

// displayURL 返回给用户看的访问地址；监听 0.0.0.0 或 :: 时展示 localhost，避免误导。
func displayURL(bc *conf.Bootstrap) string {
	host := bc.Server.HTTP.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(bc.Server.HTTP.Port))
}
