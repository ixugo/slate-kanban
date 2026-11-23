package desktop

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCleanupEndpointRequiresNativeLocalCapability(t *testing.T) {
	calls := 0
	run := func(_ context.Context, id int64, action, token string) ([]byte, error) {
		calls++
		if id != 42 || action != "cleanup-archive" || token != "approved" {
			t.Fatalf("unexpected cleanup command %d %q %q", id, action, token)
		}
		return json.Marshal(map[string]string{"status": "cleaned"})
	}
	const origin = "http://127.0.0.1:7070"
	const endpoint = "/.slate-desktop/private/cleanup"
	handler := wrapCleanup(origin, endpoint, run, http.NotFoundHandler())
	body := []byte(`{"card_id":42,"action":"cleanup-archive","preview_token":"approved"}`)
	for _, test := range []struct {
		name, method, path, origin, peer string
		status                           int
	}{
		{"browser without capability", "POST", "/api/v1/worktree/cleanup", origin, "127.0.0.1:1234", 404},
		{"remote peer", "POST", endpoint, origin, "192.0.2.1:1234", 404},
		{"wrong origin", "POST", endpoint, "http://example.test", "127.0.0.1:1234", 404},
		{"get request", "GET", endpoint, origin, "127.0.0.1:1234", 404},
		{"native approval", "POST", endpoint, origin, "127.0.0.1:1234", 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, origin+test.path, bytes.NewReader(body))
			request.Header.Set("Origin", test.origin)
			request.RemoteAddr = test.peer
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status %d, want %d: %s", response.Code, test.status, response.Body)
			}
		})
	}
	if calls != 1 {
		t.Fatalf("cleanup invoked %d times", calls)
	}
}
