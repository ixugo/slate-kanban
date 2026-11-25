package desktop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

type desktopConnection struct {
	port     int
	openPath string
}

func (c desktopConnection) script(origin string) string {
	return fmt.Sprintf(`(() => {
		if (location.origin !== %q) return;
		window.slateDesktopServer = { port: %d, openPath: %q };
		window.dispatchEvent(new Event('slate-desktop-server'));
	})()`, origin, c.port, c.openPath)
}

func (c desktopConnection) wrap(origin string, openURL func(context.Context, string) error, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != c.openPath {
			next.ServeHTTP(w, r)
			return
		}
		if !desktopLocalPeer(r) || r.Method != http.MethodPost ||
			r.Header.Get("Origin") != origin || r.Header.Get("X-Slate-Desktop") != "browser" {
			http.NotFound(w, r)
			return
		}
		if err := openURL(r.Context(), origin); err != nil {
			if r.Context().Err() != nil {
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				http.Error(w, "Browser launch timed out", http.StatusGatewayTimeout)
				return
			}
			slog.Error("无法在默认浏览器中打开 Slate", "err", err)
			http.Error(w, "Could not open Slate in a browser", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
