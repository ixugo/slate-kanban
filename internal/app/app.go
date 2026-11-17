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
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/server"
	"github.com/ixugo/goddd/pkg/system"

	"slate-kanban/internal/conf"
)

// Run 装配依赖并启动 HTTP 服务，阻塞到收到退出信号。
// 相对路径（配置、数据库、日志）都以启动时的工作目录为基准，
// 以服务方式运行时请由服务管理器设置 WorkingDirectory。
func Run(bc *conf.Bootstrap) {
	log, clean := SetupLog(bc)
	defer clean()

	// 检查是否设置了 JWT 密钥，如果未设置，则生成一个随机字符串作为密钥
	if bc.Server.HTTP.JwtSecret == "" {
		bc.Server.HTTP.JwtSecret = orm.GenerateRandomString(32)
	}

	handler, cleanUp, err := WireApp(bc, log)
	if err != nil {
		slog.Error("程序构建失败", "err", err)
		panic(err)
	}
	defer cleanUp()

	// 启动配置文件热重载
	watchCtx, watchCancel := context.WithCancel(context.Background())
	defer watchCancel()
	go conf.WatchConfig(watchCtx, bc, webhookWorkersReloader())

	addr := net.JoinHostPort(bc.Server.HTTP.Host, strconv.Itoa(bc.Server.HTTP.Port))
	svc := server.New(handler,
		server.Port(addr),
		server.ReadTimeout(bc.Server.HTTP.Timeout.Duration()),
		server.WriteTimeout(bc.Server.HTTP.Timeout.Duration()),
	)
	go svc.Start()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("服务启动成功:", displayURL(bc))

	select {
	case s := <-interrupt:
		slog.Info(`<-shutdown`, "signal", s.String())
	case err := <-svc.Notify():
		system.ErrPrintf("err: %s\n", err.Error())
		slog.Error(`<-shutdown by server.Notify`, "err", err)
	}
	if err := svc.Shutdown(); err != nil {
		slog.Error(`server.Shutdown`, "err", err)
	}
}

// SetupLog 初始化日志
func SetupLog(bc *conf.Bootstrap) (*slog.Logger, func()) {
	logDir := filepath.Join(system.Getwd(), bc.Log.Dir)

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

func webhookWorkersReloader() conf.ReloadCallback {
	return func(old, new *conf.Bootstrap) error {
		slog.Info("配置变更")
		return nil
	}
}

// displayURL 返回给用户看的访问地址；监听 0.0.0.0 或 :: 时展示 localhost，避免误导。
func displayURL(bc *conf.Bootstrap) string {
	host := bc.Server.HTTP.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(bc.Server.HTTP.Port))
}
