package desktop

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
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
	recentCards func(context.Context, int) ([]kanban.RecentCard, error)
	close       func() error
	done        <-chan struct{}
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

func startService(bc conf.Bootstrap, build app.BuildInfo, language *desktopLanguage) (*service, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(bc.Server.HTTP.Host, strconv.Itoa(bc.Server.HTTP.Port)))
	if err != nil {
		return nil, fmt.Errorf("Slate 无法监听本机服务: %w", err)
	}

	log, closeLog := app.SetupLog(&bc)
	build.Register()
	services, cleanup, err := app.WireApp(&bc, log)
	if err != nil {
		closeLog()
		return nil, errors.Join(err, listener.Close())
	}

	closing, cancel := context.WithCancel(context.Background())
	bc.Server.HTTP.Port = listener.Addr().(*net.TCPAddr).Port
	origin := strings.TrimSuffix(strings.ToLower(app.DisplayURL(&bc)), ":80")
	handler := lifecycle.Wrap(services.Handler, closing)
	if language != nil {
		handler = language.wrap(origin, handler)
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
		recentCards: services.Kanban.RecentCards,
		close:       closeService,
		done:        done,
	}, nil
}
