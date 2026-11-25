package desktop

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"sync"
)

// desktopMenuBridge reaches Slate's local WebView without reloading the
// current page or depending on the Wails asset server.
type desktopMenuBridge struct {
	path string
	tabs chan string
	mu   sync.Mutex
}

func newDesktopMenuBridge() *desktopMenuBridge {
	return &desktopMenuBridge{
		path: "/.slate-desktop/" + rand.Text() + "/menu",
		tabs: make(chan string, 1),
	}
}

// Keep the latest request if the WebView is still loading.
func (m *desktopMenuBridge) open(tab string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-m.tabs:
	default:
	}
	m.tabs <- tab
}

func (m *desktopMenuBridge) script(origin string) string {
	return fmt.Sprintf(`(() => {
		if (location.origin !== %q) return;
		const endpoint = %q;
		const source = new EventSource(endpoint);
		source.onmessage = (event) => {
			document.documentElement.dataset.slateSettingsTab = event.data;
		};
		window.addEventListener('pagehide', () => source.close(), { once: true });
		document.addEventListener('click', (event) => {
			if (!event.target.closest?.('a[data-slate-project-link]')) return;
			event.preventDefault();
			fetch(endpoint, { method: 'POST' }).catch(console.error);
		});
	})()`, origin, m.path)
}

func (m *desktopMenuBridge) wrap(closing context.Context, origin string, openProject func(context.Context) error, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != m.path {
			next.ServeHTTP(w, r)
			return
		}
		from := r.Header.Get("Origin")
		if !desktopLocalPeer(r) || (from != "" && from != origin) {
			http.NotFound(w, r)
			return
		}
		switch {
		case r.Method == http.MethodGet:
			m.stream(closing, w, r)
		case r.Method == http.MethodPost && from == origin:
			if err := openProject(r.Context()); err != nil {
				slog.Error("无法打开 Slate 的 GitHub 仓库", "err", err)
				http.Error(w, "Could not open the project in a browser", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
}

func desktopLocalPeer(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	peer = peer.Unmap()
	if peer.IsLoopback() {
		return true
	}
	// The WebView can also connect to this machine's configured interface.
	local, ok := r.Context().Value(http.LocalAddrContextKey).(*net.TCPAddr)
	return ok && !peer.IsUnspecified() && peer == local.AddrPort().Addr().Unmap()
}

func (m *desktopMenuBridge) stream(closing context.Context, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	response := http.NewResponseController(w)
	if response.Flush() != nil {
		return
	}
	for {
		select {
		case <-closing.Done():
			return
		case <-r.Context().Done():
			return
		case tab := <-m.tabs:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", tab); err != nil || response.Flush() != nil {
				return
			}
		}
	}
}
