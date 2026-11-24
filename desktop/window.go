//go:build slate_desktop && embed_web && (darwin || linux || windows)

package desktop

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"slate-kanban/internal/app"
)

const (
	displayName = "Slate"
	bundleID    = "com.ixugo.slate.desktop"
)

//go:embed icon.png
var appIcon []byte

// 图案取自网页标志的三列，64px 模板图适配 Retina 菜单栏。
//
//go:embed tray.png
var trayIcon []byte

// Run 打开 Slate 桌面窗口，退出后返回更新请求指定的重启路径。
func Run(build app.BuildInfo) (string, error) {
	cfg, closeConfig, cfgErr := loadConfig(build.Version)
	if closeConfig != nil {
		defer closeConfig()
	}
	language := newDesktopLanguage(cfg.Runtime.ConfigDir)
	var native *application.App
	var window *application.WebviewWindow
	show := func() {
		if window == nil {
			return
		}
		if window.IsMinimised() {
			window.Restore()
		}
		if runtime.GOOS == "darwin" {
			native.Show()
		}
		window.Show()
		window.Focus()
	}
	native = application.New(application.Options{
		Name:        displayName,
		Description: "Slate",
		Icon:        appIcon,
		Windows: application.WindowsOptions{
			WebviewUserDataPath: filepath.Join(cfg.Runtime.ConfigDir, "webview"),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               bundleID,
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { show() },
		},
	})
	if cfgErr != nil {
		return "", showStartupError(native, cfgErr, language.text())
	}

	if runtime.GOOS == "darwin" {
		cfg.Runtime.OpenUpdateInstaller = func(ctx context.Context, path string) error {
			return exec.CommandContext(ctx, "/usr/bin/open", path).Run()
		}
	}
	restart := make(chan string, 1)
	cfg.Runtime.RequestRestart = func(exe string) bool {
		select {
		case restart <- exe:
		default:
		}
		return true
	}
	s, err := startService(cfg, build, language)
	if err != nil {
		return "", showStartupError(native, err, language.text())
	}

	native.OnShutdown(func() {
		if err := s.close(); err != nil {
			slog.Error("关闭 Slate 服务失败", "err", err)
		}
		closeConfig()
	})
	window = native.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:               "main",
		Title:              displayName,
		URL:                s.url,
		JS:                 s.cleanup.script(s.url) + ";\n" + language.script(s.url) + ";\n" + s.dispatch.script(s.url),
		Width:              1280,
		Height:             820,
		MinWidth:           760,
		MinHeight:          520,
		UseApplicationMenu: true,
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHidden,
			InvisibleTitleBarHeight: 28,
			WebviewPreferences: application.MacWebviewPreferences{
				ApplicationNameForUserAgent: "SlateDesktop",
			},
		},
	})
	if runtime.GOOS == "darwin" {
		window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
			event.Cancel()
			window.Hide()
		})
		native.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) { show() })
		addTray(native, s, window, show, language)
	}
	updateMenu := func() {
		menu := applicationMenu(native, window, s.url, show, language.text())
		native.Menu.Set(menu)
		if runtime.GOOS != "darwin" {
			window.SetMenu(menu)
		}
	}
	updateMenu()

	finished := make(chan struct{})
	restartPath := make(chan string, 1)
	native.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			for {
				select {
				case exe := <-restart:
					restartPath <- exe
					native.Quit()
					return
				case <-s.done:
					native.Quit()
					return
				case <-language.changed:
					application.InvokeSync(updateMenu)
				case <-finished:
					return
				}
			}
		}()
	})
	runErr := native.Run()
	close(finished)
	var exe string
	select {
	case exe = <-restartPath:
	default:
	}
	return exe, errors.Join(runErr, s.close())
}

func showStartupError(native *application.App, err error, text desktopText) error {
	message := text.startupMessage(err)
	native.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		dialog := native.Dialog.Error().SetTitle(displayName).SetMessage(message)
		dialog.AddButton(text.confirm).OnClick(native.Quit)
		dialog.Show()
	})
	return errors.Join(err, native.Run())
}

func applicationMenu(native *application.App, window *application.WebviewWindow, origin string, show func(), text desktopText) *application.Menu {
	menu := application.NewMenu()
	if runtime.GOOS == "darwin" {
		appMenu := menu.AddSubmenu(displayName)
		addRole(appMenu, application.About, text.about)
		appMenu.AddSeparator()
		addRole(appMenu, application.Hide, text.hide)
		addRole(appMenu, application.HideOthers, text.hideOthers)
		addRole(appMenu, application.UnHide, text.showAll)
		appMenu.AddSeparator()
		addRole(appMenu, application.Quit, text.quit)
	}
	navigation := menu.AddSubmenu(text.navigate)
	navigation.Add(text.show).OnClick(func(*application.Context) { show() })
	navigation.Add(text.boards).OnClick(func(*application.Context) {
		window.SetURL(origin)
		show()
	})
	navigation.Add(text.agents).OnClick(func(*application.Context) {
		window.SetURL(origin + "/agents")
		show()
	})
	if runtime.GOOS != "darwin" {
		navigation.AddSeparator()
		navigation.Add(text.quit).OnClick(func(*application.Context) { native.Quit() })
	}

	edit := menu.AddSubmenu(text.edit)
	addRole(edit, application.Undo, text.undo)
	addRole(edit, application.Redo, text.redo)
	edit.AddSeparator()
	addRole(edit, application.Cut, text.cut)
	addRole(edit, application.Copy, text.copy)
	addRole(edit, application.Paste, text.paste)
	addRole(edit, application.SelectAll, text.selectAll)
	return menu
}

func addRole(menu *application.Menu, role application.Role, label string) {
	menu.AddRole(role)
	if item := menu.FindByRole(role); item != nil {
		item.SetLabel(label)
	}
}

func addTray(native *application.App, s *service, window *application.WebviewWindow, show func(), language *desktopLanguage) {
	tray := native.SystemTray.New().SetTemplateIcon(trayIcon)
	tray.SetTooltip("Slate")
	menu := application.NewMenu()
	tray.SetMenu(menu)
	openMenu := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cards, err := s.recentCards(ctx, 8)
		text := language.text()
		menu.Clear()
		switch {
		case err != nil:
			slog.Warn("读取状态栏最近任务失败", "err", err)
			menu.Add(text.unavailable).SetEnabled(false)
		case len(cards) == 0:
			menu.Add(text.empty).SetEnabled(false)
		default:
			for _, card := range cards {
				menu.Add(card.Key + "  " + card.Title).OnClick(func(*application.Context) {
					window.SetURL(fmt.Sprintf("%s/b/%d?card=%d", s.url, card.BoardID, card.ID))
					show()
				})
			}
		}
		menu.AddSeparator()
		menu.Add(text.quit).OnClick(func(*application.Context) { native.Quit() })
		menu.Update()
		tray.OpenMenu()
	}
	tray.OnClick(openMenu).OnRightClick(openMenu)
}
