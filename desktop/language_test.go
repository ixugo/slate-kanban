package desktop

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDesktopLanguageFollowsWebView(t *testing.T) {
	lang := newDesktopLanguage(t.TempDir())
	const origin = "http://127.0.0.1:7070"
	handler := lang.wrap(origin, http.NotFoundHandler())
	send := func(from, code string, want int) {
		t.Helper()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+lang.path, strings.NewReader(code))
		req.Header.Set("Origin", from)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("origin %s, language %s: %d, want %d", from, code, w.Code, want)
		}
	}
	send("http://elsewhere.invalid", "ko", http.StatusForbidden)
	send(origin, "unsupported", http.StatusBadRequest)
	for _, code := range []string{"ja", "ko", "zh-TW", "en", "zh-CN"} {
		send(origin, code, http.StatusNoContent)
		if got := lang.text(); got != desktopTexts[code] || got.startup == "" || got.inUse == "" || got.show == "" {
			t.Errorf("桌面语言未同步或缺少文案: %s %+v", code, got)
		}
	}
	saved, err := os.ReadFile(lang.file)
	if err != nil || string(saved) != "zh-CN" {
		t.Errorf("未保存最后一次界面语言: %q (%v)", saved, err)
	}
}

func TestSystemLanguageFallback(t *testing.T) {
	for _, test := range []struct {
		candidates []string
		want       string
	}{
		{[]string{"fr-FR", "zh_Hant_HK"}, "zh-TW"},
		{[]string{"zh_CN.UTF-8"}, "zh-CN"},
		{[]string{"ja-JP"}, "ja"},
		{[]string{"ko-KR"}, "ko"},
		{[]string{"en-AU", "ja"}, "en"},
		{[]string{"es", "fr"}, "en"},
	} {
		if got := preferredLanguage(test.candidates); got != test.want {
			t.Errorf("%v: got %s, want %s", test.candidates, got, test.want)
		}
	}
}
