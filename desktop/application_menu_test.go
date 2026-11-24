package desktop

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDesktopMenuDeliversPendingAndLiveRequests(t *testing.T) {
	menu := newDesktopMenuBridge()
	closing, stop := context.WithCancel(t.Context())
	server := httptest.NewServer(menu.wrap(closing, "http://127.0.0.1", nil, http.NotFoundHandler()))
	t.Cleanup(server.Close)
	t.Cleanup(stop)
	menu.open("general")
	menu.open("about")

	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(server.URL + menu.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	if contentType := response.Header.Get("Content-Type"); response.StatusCode != http.StatusOK || contentType != "text/event-stream" {
		t.Fatalf("status = %d, content type = %q", response.StatusCode, contentType)
	}
	lines := bufio.NewScanner(response.Body)
	assertTab := func(want string) {
		t.Helper()
		if !lines.Scan() || lines.Text() != "data: "+want || !lines.Scan() || lines.Text() != "" {
			t.Fatalf("expected complete event for %q, last line = %q, err = %v", want, lines.Text(), lines.Err())
		}
	}
	assertTab("about")
	menu.open("general")
	assertTab("general")
	stop()
	if lines.Scan() || lines.Err() != nil {
		t.Fatalf("expected the stream to close on shutdown, last line = %q, err = %v", lines.Text(), lines.Err())
	}
}

func TestDesktopMenuRequiresLocalPeerAndSameOrigin(t *testing.T) {
	const origin = "http://192.0.2.20:7070"
	for _, tt := range []struct {
		name, remote, local, from string
		wantGET, wantPOST         int
	}{
		{name: "loopback", remote: "127.0.0.1:1200", from: origin, wantGET: http.StatusOK, wantPOST: http.StatusNoContent},
		{name: "same interface", remote: "192.0.2.20:1200", local: "192.0.2.20", from: origin, wantGET: http.StatusOK, wantPOST: http.StatusNoContent},
		{name: "mapped IPv4", remote: "[::ffff:192.0.2.20]:1200", local: "192.0.2.20", from: origin, wantGET: http.StatusOK, wantPOST: http.StatusNoContent},
		{name: "IPv6", remote: "[2001:db8::20]:1200", local: "2001:db8::20", from: origin, wantGET: http.StatusOK, wantPOST: http.StatusNoContent},
		{name: "foreign origin", remote: "192.0.2.20:1200", local: "192.0.2.20", from: "https://other.example", wantGET: http.StatusNotFound, wantPOST: http.StatusNotFound},
		{name: "missing origin", remote: "192.0.2.20:1200", local: "192.0.2.20", wantGET: http.StatusOK, wantPOST: http.StatusNotFound},
		{name: "remote peer", remote: "192.0.2.5:1200", local: "192.0.2.20", from: origin, wantGET: http.StatusNotFound, wantPOST: http.StatusNotFound},
		{name: "missing listener address", remote: "192.0.2.20:1200", from: origin, wantGET: http.StatusNotFound, wantPOST: http.StatusNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			menu := newDesktopMenuBridge()
			closing, stop := context.WithCancel(t.Context())
			stop() // The GET still writes its HTTP status before the stream shuts down.
			opened := false
			handler := menu.wrap(closing, origin, func(context.Context) error {
				opened = true
				return nil
			}, http.NotFoundHandler())
			ctx := t.Context()
			if tt.local != "" {
				ctx = context.WithValue(ctx, http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP(tt.local), Port: 7070})
			}
			for _, method := range []struct {
				name string
				want int
			}{{http.MethodGet, tt.wantGET}, {http.MethodPost, tt.wantPOST}} {
				req := httptest.NewRequestWithContext(ctx, method.name, origin+menu.path, nil)
				req.RemoteAddr = tt.remote
				req.Header.Set("Origin", tt.from)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				if response.Code != method.want || opened != (method.want == http.StatusNoContent) {
					t.Errorf("%s status = %d, browser opened = %t", method.name, response.Code, opened)
				}
			}
		})
	}
}
