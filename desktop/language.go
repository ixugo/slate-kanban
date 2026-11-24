package desktop

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"slate-kanban/internal/app"
	"slate-kanban/internal/core/kanban"
)

type desktopText struct {
	navigate, show, boards, agents, quit          string
	empty, unavailable, startup, inUse, confirm   string
	listenUnavailable                             string
	edit, undo, redo, cut, copy, paste, selectAll string
	about, hide, hideOthers, showAll              string
}

var desktopTexts = map[string]desktopText{
	"en": {
		navigate: "Navigate", show: "Show Slate", boards: "Boards", agents: "Agent Setup", quit: "Quit Slate",
		empty: "No recent cards", unavailable: "Recent cards unavailable", startup: "Slate could not start. Check its configuration and try again.",
		inUse: "Slate is already using this configuration. Quit the running server or desktop app, then open Slate again.", confirm: "OK",
		listenUnavailable: "Slate could not listen on %s. Check the listening address and port in config.toml. If the port is in use, quit the program using it before reopening Slate.",
		edit:              "Edit", undo: "Undo", redo: "Redo", cut: "Cut", copy: "Copy", paste: "Paste", selectAll: "Select All",
		about: "About Slate", hide: "Hide Slate", hideOthers: "Hide Others", showAll: "Show All",
	},
	"zh-CN": {
		navigate: "导航", show: "显示 Slate", boards: "看板", agents: "接入 Agent", quit: "退出 Slate",
		empty: "暂无最近任务", unavailable: "无法读取最近任务", startup: "Slate 无法启动。请检查配置后重试。",
		inUse: "已有 Slate 进程在使用此配置。请退出运行中的服务端或桌面应用，再打开 Slate。", confirm: "确定",
		listenUnavailable: "Slate 无法监听 %s。请检查 config.toml 中的监听地址和端口。如果端口已被占用，请退出占用该端口的程序，再打开 Slate。",
		edit:              "编辑", undo: "撤销", redo: "重做", cut: "剪切", copy: "复制", paste: "粘贴", selectAll: "全选",
		about: "关于 Slate", hide: "隐藏 Slate", hideOthers: "隐藏其他", showAll: "显示全部",
	},
	"zh-TW": {
		navigate: "導覽", show: "顯示 Slate", boards: "看板", agents: "連接 Agent", quit: "結束 Slate",
		empty: "目前沒有最近任務", unavailable: "無法讀取最近任務", startup: "Slate 無法啟動。請檢查設定後再試一次。",
		inUse: "已有 Slate 程序正在使用此設定。請結束執行中的伺服器或桌面應用程式，再開啟 Slate。", confirm: "確定",
		listenUnavailable: "Slate 無法監聽 %s。請檢查 config.toml 中的監聽位址和連接埠。如果連接埠已被占用，請結束占用該連接埠的程式，再開啟 Slate。",
		edit:              "編輯", undo: "復原", redo: "重做", cut: "剪下", copy: "複製", paste: "貼上", selectAll: "全選",
		about: "關於 Slate", hide: "隱藏 Slate", hideOthers: "隱藏其他", showAll: "顯示全部",
	},
	"ja": {
		navigate: "移動", show: "Slate を表示", boards: "ボード", agents: "エージェント設定", quit: "Slate を終了",
		empty: "最近のカードはありません", unavailable: "最近のカードを読み込めません", startup: "Slate を起動できませんでした。設定を確認してからやり直してください。",
		inUse: "この設定は別の Slate で使用中です。実行中のサーバーまたはデスクトップアプリを終了し、Slate を開き直してください。", confirm: "OK",
		listenUnavailable: "Slate は %s で待ち受けできませんでした。config.toml の待ち受けアドレスとポートを確認してください。ポートが使用中の場合は、そのアプリを終了してから Slate を開き直してください。",
		edit:              "編集", undo: "取り消す", redo: "やり直す", cut: "切り取り", copy: "コピー", paste: "貼り付け", selectAll: "すべてを選択",
		about: "Slate について", hide: "Slate を隠す", hideOthers: "ほかを隠す", showAll: "すべてを表示",
	},
}

func (t desktopText) startupMessage(err error) string {
	if errors.Is(err, app.ErrConfigInUse) {
		return t.inUse
	}
	if listenErr, ok := errors.AsType[*net.OpError](err); ok && listenErr.Op == "listen" && listenErr.Addr != nil {
		return fmt.Sprintf(t.listenUnavailable, listenErr.Addr)
	}
	return t.startup
}

func preferredLanguage(candidates []string) string {
	for _, tag := range candidates {
		tag, _, _ = strings.Cut(tag, ".")
		tag = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
		base, _, _ := strings.Cut(tag, "-")
		switch base {
		case "en", "zh", "ja":
			return kanban.NormalizeLocale(tag)
		}
	}
	return "en"
}

// desktopLanguage 跟随 WebView 已有的界面语言，并保存原生菜单的最后一次选择，
// 这样下次启动失败时，原生提示也能用上次的语言。
type desktopLanguage struct {
	file    string
	path    string
	current atomic.Pointer[string]
	changed chan struct{}
}

func newDesktopLanguage(dir string) *desktopLanguage {
	lang := &desktopLanguage{path: "/.slate-desktop/" + rand.Text(), changed: make(chan struct{}, 1)}
	if dir != "" {
		lang.file = filepath.Join(dir, "desktop-language")
	}
	saved, _ := os.ReadFile(lang.file)
	code := strings.TrimSpace(string(saved))
	if _, ok := desktopTexts[code]; !ok {
		code = preferredLanguage(systemLanguages())
	}
	lang.current.Store(new(code))
	return lang
}

func (l *desktopLanguage) text() desktopText {
	return desktopTexts[*l.current.Load()]
}

// script runs only in the native window on the configured Slate origin.
// It observes the same language that React has already chosen and applied to HTML.
func (l *desktopLanguage) script(origin string) string {
	return fmt.Sprintf(`(() => {
		if (location.origin !== %q) return;
		let last;
		let requests = Promise.resolve();
		const send = () => {
			const language = document.documentElement.lang;
			if (!language || language === last) return;
			last = language;
			requests = requests.then(() => fetch(%q, {
				method: 'POST', body: language, keepalive: true
			})).catch(() => {});
		};
		new MutationObserver(send).observe(document.documentElement, {attributeFilter: ['lang']});
		send();
	})()`, origin, l.path)
}

// wrap installs a desktop-only endpoint. The random path is sent only to the
// native WebView, and same-origin requests alone may update the locale.
func (l *desktopLanguage) wrap(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != l.path {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodPost || r.Header.Get("Origin") != origin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 12))
		code := string(raw)
		if _, ok := desktopTexts[code]; err != nil || !ok {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if code != *l.current.Swap(new(code)) {
			if err := os.WriteFile(l.file, raw, 0o600); err != nil {
				slog.Warn("保存桌面语言失败", "err", err)
			}
			select {
			case l.changed <- struct{}{}:
			default:
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
