package desktop

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"slate-kanban/internal/app"
	"slate-kanban/internal/conf"
	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/lifecycle"
)

type service struct {
	url         string
	connection  desktopConnection
	recentCards func(context.Context, int) ([]kanban.RecentCard, error)
	dispatch    *desktopDispatch
	menu        *desktopMenuBridge
	close       func() error
	done        <-chan struct{}
	cleanup     worktreeCleanup
}

func loadConfig(version string) (conf.Bootstrap, func(), error) {
	dir, err := conf.DefaultDir()
	if err != nil {
		return conf.Bootstrap{}, nil, err
	}
	bc, closeConfig, err := app.OpenConfig(dir)
	bc.Runtime.ConfigDir = dir
	bc.Runtime.BuildVersion = version
	bc.Runtime.Desktop = true
	return bc, closeConfig, err
}

func startService(bc conf.Bootstrap, build app.BuildInfo, language *desktopLanguage, openURL func(context.Context, string) error) (*service, error) {
	host, port := bc.Server.HTTP.Host, bc.Server.HTTP.Port
	listener, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if port != 0 && errors.Is(err, errAddressInUse) {
		listener, err = net.Listen("tcp", net.JoinHostPort(host, "0"))
	}
	if err != nil {
		return nil, fmt.Errorf("Slate 无法监听本机服务: %w", err)
	}
	bc.Server.HTTP.Port = listener.Addr().(*net.TCPAddr).Port

	log, closeLog := app.SetupLog(&bc)
	build.Register()
	services, cleanup, err := app.WireApp(&bc, log)
	if err != nil {
		closeLog()
		return nil, errors.Join(err, listener.Close())
	}

	closing, cancel := context.WithCancel(context.Background())
	origin := strings.TrimSuffix(strings.ToLower(app.DisplayURL(&bc)), ":80")
	handler := lifecycle.Wrap(services.Handler, closing)
	connection := desktopConnection{port: bc.Server.HTTP.Port}
	if openURL != nil {
		connection.openPath = "/.slate-desktop/" + rand.Text() + "/browser"
		handler = connection.wrap(origin, openURL, handler)
	}
	localCleanup := newWorktreeCleanup()
	var dispatch *desktopDispatch
	var menu *desktopMenuBridge
	if language != nil {
		handler = language.wrap(origin, handler)
		handler = wrapCleanup(origin, localCleanup.path, runWorktreeCleanup(services.Kanban), handler)
		dispatch = newDesktopDispatch(services.Kanban, bc.Runtime.ConfigDir, language.text)
		handler = dispatch.wrap(origin, handler)
		if runtime.GOOS == "darwin" {
			menu = newDesktopMenuBridge()
			handler = menu.wrap(closing, origin, func(ctx context.Context) error {
				return exec.CommandContext(ctx, "/usr/bin/open", "https://github.com/ixugo/slate-kanban").Run()
			}, handler)
		}
	}
	httpServer := &http.Server{
		Handler:           handler,
		ReadTimeout:       bc.Server.HTTP.Timeout.Duration(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	done := make(chan struct{})
	var serveErr error
	go func() {
		defer close(done)
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
		}
	}()

	closeService := sync.OnceValue(func() error {
		cancel()
		if dispatch != nil {
			dispatch.close()
		}
		ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		err := httpServer.Shutdown(ctx)
		if err != nil {
			err = errors.Join(err, httpServer.Close())
		}
		<-done
		cleanup()
		closeLog()
		return errors.Join(serveErr, err)
	})

	return &service{
		url:         origin,
		connection:  connection,
		recentCards: services.Kanban.RecentCards,
		dispatch:    dispatch,
		menu:        menu,
		close:       closeService,
		done:        done,
		cleanup:     localCleanup,
	}, nil
}
