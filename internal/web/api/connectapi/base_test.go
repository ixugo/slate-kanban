package connectapi

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ixugo/goddd/pkg/reason"
)

// TestBaseURL 验证服务地址是怎么从请求里算出来的：协议看转发头和 TLS，主机看转发头和 Host，
// 只认合法的主机名（可以带端口，IPv6 要有方括号）；不合法的一律拒绝，因为这个地址会被写进用户要复制执行的命令里。
func TestBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		headers map[string]string
		tls     bool
		want    string // 为空表示必须被拒绝
	}{
		{name: "本机带端口", host: "localhost:7070", want: "http://localhost:7070"},
		{name: "域名不带端口", host: "kanban.example.com", want: "http://kanban.example.com"},
		{name: "IPv4 带端口", host: "127.0.0.1:7070", want: "http://127.0.0.1:7070"},
		{name: "IPv6 带端口", host: "[::1]:7070", want: "http://[::1]:7070"},
		{name: "容器服务名带下划线", host: "slate_kanban:7070", want: "http://slate_kanban:7070"},
		{name: "TLS 直连", host: "kanban.example.com", tls: true, want: "https://kanban.example.com"},
		{name: "转发协议 https", host: "kanban.example.com", headers: map[string]string{"X-Forwarded-Proto": "https"}, want: "https://kanban.example.com"},
		{name: "转发协议大写", host: "kanban.example.com", headers: map[string]string{"X-Forwarded-Proto": "HTTPS"}, want: "https://kanban.example.com"},
		{name: "转发协议多个值取第一个", host: "kanban.example.com", headers: map[string]string{"X-Forwarded-Proto": "https, http"}, want: "https://kanban.example.com"},
		{name: "转发协议不认识就忽略", host: "kanban.example.com", headers: map[string]string{"X-Forwarded-Proto": "javascript"}, want: "http://kanban.example.com"},
		{name: "转发协议盖过 TLS 直连", host: "kanban.example.com", tls: true, headers: map[string]string{"X-Forwarded-Proto": "http"}, want: "http://kanban.example.com"},
		{name: "转发主机优先", host: "internal:7070", headers: map[string]string{"X-Forwarded-Host": "kanban.example.com", "X-Forwarded-Proto": "https"}, want: "https://kanban.example.com"},
		{name: "转发主机多个值取第一个", host: "internal", headers: map[string]string{"X-Forwarded-Host": "a.example.com, b.example.com"}, want: "http://a.example.com"},
		{name: "端口上限", host: "kanban:65535", want: "http://kanban:65535"},
		{name: "转发主机只有空白就当作没有", host: "kanban.example.com", headers: map[string]string{"X-Forwarded-Host": " , "}, want: "http://kanban.example.com"},

		{name: "没有 Host", host: "", headers: nil},
		{name: "Host 带路径", host: "evil.com/path"},
		{name: "Host 带空格", host: "evil .com"},
		{name: "Host 带引号", host: `evil.com"onload="x`},
		{name: "Host 带尖括号", host: "<script>.com"},
		{name: "Host 带用户信息", host: "user@evil.com"},
		{name: "Host 带分号", host: "evil.com;rm"},
		{name: "标签以连字符开头", host: "-evil.com"},
		{name: "标签为空", host: "evil..com"},
		{name: "端口为空", host: "kanban:"},
		{name: "端口不是数字", host: "kanban:80a"},
		{name: "端口超过上限", host: "kanban:65536"},
		{name: "端口为零", host: "kanban:0"},
		{name: "Host 太长", host: strings.Repeat("a", maxHostLen+1)},
		{name: "转发主机不合法时不回退到 Host", host: "kanban.example.com", headers: map[string]string{"X-Forwarded-Host": "evil.com/x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/install.md", http.NoBody)
			r.Host = tc.host
			for k, v := range tc.headers {
				r.Header.Set(k, v)
			}
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			got, err := baseURL(r)
			if tc.want == "" {
				assertBadRequest(t, got, err)
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("期望 %q，实际 %q %v", tc.want, got, err)
			}
		})
	}
}

// assertBadRequest 验证请求被拒绝：没有返回地址，错误是状态码 400 的 ErrBadRequest。
func assertBadRequest(t *testing.T, got string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("应该被拒绝，却得到 %q", got)
	}
	var info reason.ErrorInfoer
	if !errors.As(err, &info) || got != "" || info.GetHTTPCode() != http.StatusBadRequest || info.GetReason() != reason.ErrBadRequest.GetReason() {
		t.Fatalf("应该返回 400 的 ErrBadRequest 并且不带地址: %q %v", got, err)
	}
}
