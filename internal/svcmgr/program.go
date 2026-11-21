package svcmgr

import (
	"os"
	"runtime"
	"time"

	"github.com/kardianos/service"
)

// stopTimeout 是服务停止时等待程序优雅退出的最长时间，超时就放弃等待，避免服务管理器一直卡在「正在停止」。
const stopTimeout = 30 * time.Second

// Program 实现 service.Interface，把 Slate 的主流程接到服务管理器上。
type Program struct {
	run  func(stop <-chan struct{})
	stop chan struct{}
	done chan struct{}
}

// UnderServiceManager 判断是否由 Windows 服务管理器拉起。
// Linux 和 macOS 的 systemd/launchd 只是普通地启动进程并发信号，原有的信号处理就够用，不需要走服务接口。
func UnderServiceManager() bool {
	return runtime.GOOS == "windows" && !service.Interactive()
}

// RunAsService 以 Windows 服务的方式运行 run，阻塞到服务被停止；run 收到 stop 关闭后应当优雅退出。
func RunAsService(run func(stop <-chan struct{})) error {
	p := &Program{run: run, stop: make(chan struct{}), done: make(chan struct{})}
	svc, err := service.New(p, &service.Config{Name: serviceName})
	if err != nil {
		return err
	}
	return svc.Run()
}

// Start 在后台启动主流程；服务管理器要求它马上返回。
// 主流程自己退出（比如监听失败）时，进程以非 0 退出，让服务管理器知道服务已经挂了。
func (p *Program) Start(service.Service) error {
	go func() {
		p.run(p.stop)
		close(p.done)
		select {
		case <-p.stop:
		default:
			os.Exit(1)
		}
	}()
	return nil
}

// Stop 通知主流程退出，并等它优雅关闭（最多 stopTimeout）。
func (p *Program) Stop(service.Service) error {
	close(p.stop)
	select {
	case <-p.done:
	case <-time.After(stopTimeout):
	}
	return nil
}
