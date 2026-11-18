package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/conf"
)

const testPasscode = "test-pass-123456"

// miniPasscodeEngine 挂口令中间件和少量路由，便于单测而不必起完整依赖。
func miniPasscodeEngine(passcode string, limiter *attemptLimiter) *gin.Engine {
	if limiter == nil {
		limiter = newAttemptLimiter(nil)
	}
	bc := &conf.Bootstrap{Server: conf.Server{HTTP: conf.ServerHTTP{Passcode: passcode}}}
	guard := newPasscodeGuard(bc, limiter)
	g := gin.New()
	g.Use(passcodeMiddleware(guard))
	registerAuthRoutes(g, guard)
	g.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	g.GET("/install.md", func(c *gin.Context) { c.Status(http.StatusOK) })
	g.GET("/skills/slate-kanban/SKILL.md", func(c *gin.Context) { c.Status(http.StatusOK) })
	g.GET("/api/v1/boards", func(c *gin.Context) { c.Status(http.StatusOK) })
	g.GET("/app/metrics/api", func(c *gin.Context) { c.Status(http.StatusOK) })
	g.POST("/mcp", func(c *gin.Context) { c.Status(http.StatusOK) })
	g.GET("/nope-static", func(c *gin.Context) { c.Status(http.StatusNotFound) })
	return g
}

// postJSON 构造带 JSON 请求体的 POST，并写上 Content-Length 以便 goddd 的 WrapH 能绑定。
func postJSON(path, body string) *http.Request {
	raw := []byte(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(raw))
	return req
}

// serve 用 httptest 发请求并带上可选头与 cookie。
func serve(t *testing.T, g *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

// TestPasscodeDisabledAllowsAPI 未配置口令时受保护路径应直通。
func TestPasscodeDisabledAllowsAPI(t *testing.T) {
	g := miniPasscodeEngine("", nil)
	rec := serve(t, g, httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("无口令应放行: %d %s", rec.Code, rec.Body)
	}
}

// TestPasscodeRequiredWithoutCredential401 有口令且无凭证时返回 401。
func TestPasscodeRequiredWithoutCredential401(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	for _, path := range []string{"/api/v1/boards", "/app/metrics/api", "/mcp"} {
		rec := serve(t, g, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s 应 401: %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "ErrUnauthorized") {
			t.Fatalf("应含 ErrUnauthorized: %s", rec.Body)
		}
		if path == "/mcp" && rec.Header().Get("WWW-Authenticate") == "" {
			t.Fatal("MCP 401 应带 WWW-Authenticate")
		}
	}
}

// TestPasscodeBearerAllows 正确 Bearer 应放行。
func TestPasscodeBearerAllows(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil)
	req.Header.Set("Authorization", "Bearer "+testPasscode)
	rec := serve(t, g, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Bearer 应通过: %d %s", rec.Code, rec.Body)
	}
}

// TestPasscodeWrongBearer401 错误 Bearer 记失败并返回 401。
func TestPasscodeWrongBearer401(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := serve(t, g, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("错误 Bearer 应 401: %d", rec.Code)
	}
}

// TestPasscodePublicPaths 公开路径不需要口令。
func TestPasscodePublicPaths(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	for _, path := range []string{"/health", "/install.md", "/skills/slate-kanban/SKILL.md", "/nope-static"} {
		rec := serve(t, g, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code == http.StatusUnauthorized {
			t.Fatalf("%s 不应要求口令: %d", path, rec.Code)
		}
	}
}

// TestAuthStatusThreeStates 覆盖 status 的三种语义。
func TestAuthStatusThreeStates(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	rec := serve(t, g, httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil))
	var st authStatusResp
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Required || st.Authenticated {
		t.Fatalf("应 required=true, authenticated=false: %+v", st)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	req.Header.Set("Authorization", "Bearer "+testPasscode)
	rec = serve(t, g, req)
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Required || !st.Authenticated {
		t.Fatalf("Bearer 后应 authenticated=true: %+v", st)
	}
	g2 := miniPasscodeEngine("", nil)
	rec = serve(t, g2, httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Required || !st.Authenticated {
		t.Fatalf("未启用口令应 required=false, authenticated=true: %+v", st)
	}
}

// TestLoginLogoutCookieFlow 登录种 cookie、带 cookie 访问、登出清 cookie。
func TestLoginLogoutCookieFlow(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	rec := serve(t, g, postJSON("/api/v1/auth/login", `{"passcode":"`+testPasscode+`"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("登录应 200: %d %s", rec.Code, rec.Body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != sessionCookieName {
		t.Fatal("登录应 Set-Cookie")
	}
	apiReq := httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil)
	for _, c := range cookies {
		apiReq.AddCookie(c)
	}
	rec = serve(t, g, apiReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("带 cookie 应通过: %d", rec.Code)
	}
	logoutRec := serve(t, g, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	if logoutRec.Code != http.StatusOK {
		t.Fatal("logout 应 200")
	}
}

// TestSessionTamperInvalid 篡改或过期会话应 401。
func TestSessionTamperInvalid(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	bad := httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil)
	bad.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "v1.0.bad"})
	rec := serve(t, g, bad)
	if rec.Code != http.StatusUnauthorized {
		t.Fatal("篡改 cookie 应 401")
	}
	exp := time.Now().Add(-time.Hour).Unix()
	val := signedSessionValue(testPasscode, exp)
	old := httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil)
	old.AddCookie(&http.Cookie{Name: sessionCookieName, Value: val})
	rec = serve(t, g, old)
	if rec.Code != http.StatusUnauthorized {
		t.Fatal("过期 cookie 应 401")
	}
	other := signedSessionValue("other-passcode-12345", time.Now().Add(time.Hour).Unix())
	wrongKey := httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil)
	wrongKey.AddCookie(&http.Cookie{Name: sessionCookieName, Value: other})
	rec = serve(t, g, wrongKey)
	if rec.Code != http.StatusUnauthorized {
		t.Fatal("换口令签的 cookie 应 401")
	}
}

// signedSessionValue 按与生产相同算法签发测试用会话值。
func signedSessionValue(passcode string, exp int64) string {
	payload := fmt.Sprintf("%s.%d", sessionFormatVersion, exp)
	mac := hmac.New(sha256.New, sessionSigningKey(passcode))
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// TestLoginValidation400 登录体空或超长应 400。
func TestLoginValidation400(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	for _, body := range []string{`{"passcode":""}`, `{"passcode":"` + strings.Repeat("x", conf.PasscodeMaxLen+1) + `"}`} {
		rec := serve(t, g, postJSON("/api/v1/auth/login", body))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("应 400: %d %s", rec.Code, body)
		}
	}
}

// TestSecureCookieOnHTTPS 在 X-Forwarded-Proto 为 https 时 cookie 带 Secure。
func TestSecureCookieOnHTTPS(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	req := postJSON("/api/v1/auth/login", `{"passcode":"`+testPasscode+`"}`)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := serve(t, g, req)
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName && !c.Secure {
			t.Fatal("Secure 应为 true")
		}
	}
}

// TestRateLimit429 连续错误口令后应 429 且带 Retry-After。
func TestRateLimit429(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	limiter := newAttemptLimiter(func() time.Time { return now })
	g := miniPasscodeEngine(testPasscode, limiter)
	ip := "203.0.113.9"
	for i := 0; i < 11; i++ {
		req := postJSON("/api/v1/auth/login", `{"passcode":"wrong-`+fmt.Sprint(i)+`"}`)
		req.RemoteAddr = ip + ":1234"
		serve(t, g, req)
	}
	req := postJSON("/api/v1/auth/login", `{"passcode":"`+testPasscode+`"}`)
	req.RemoteAddr = ip + ":1234"
	rec := serve(t, g, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("封锁期内正确口令也应 429: %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("应带 Retry-After")
	}
}

// TestPasscodeMiddlewareRequired 证明启用口令且挂中间件后，无凭证不能访问受保护路径。
func TestPasscodeMiddlewareRequired(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil)
	rec := serve(t, g, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatal("挂中间件后无凭证应 401")
	}
}

// withBearer 给请求加上 Bearer 头并指定来源地址，便于模拟不同 IP 的猜口令请求。
func withBearer(req *http.Request, token, remote string) *http.Request {
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = remote + ":4321"
	return req
}

// failTimes 用错误 Bearer 连续访问受保护接口 n 次，模拟一个 IP 在猜口令。
func failTimes(t *testing.T, g *gin.Engine, remote string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		req := withBearer(httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil), fmt.Sprintf("guess-%d", i), remote)
		if rec := serve(t, g, req); rec.Code != http.StatusUnauthorized && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("错误口令应 401 或 429: %d", rec.Code)
		}
	}
}

// TestBearerGuessingIsBlockedEvenWithCorrectPasscode 猜错超过上限后，该 IP 连正确口令的 Bearer 也要 429。
func TestBearerGuessingIsBlockedEvenWithCorrectPasscode(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	const ip = "203.0.113.20"
	failTimes(t, g, ip, maxFailedAttemptsPerWindow+1)
	rec := serve(t, g, withBearer(httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil), testPasscode, ip))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("封锁期内正确 Bearer 也应 429 并带 Retry-After: %d", rec.Code)
	}
	rec = serve(t, g, withBearer(httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil), testPasscode, "203.0.113.21"))
	if rec.Code != http.StatusOK {
		t.Fatalf("其他 IP 不受影响: %d", rec.Code)
	}
}

// TestStaleCookieNeverTripsLimiter 浏览器里留着失效 cookie（比如改过口令）时，页面连续请求不能把自己锁死，
// 之后输对口令仍能登录。cookie 靠签名防伪，猜不出来，不需要限流。
func TestStaleCookieNeverTripsLimiter(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	const ip = "203.0.113.30"
	stale := &http.Cookie{Name: sessionCookieName, Value: signedSessionValue("old-passcode-123456", time.Now().Add(time.Hour).Unix())}
	for i := 0; i < maxFailedAttemptsPerWindow*3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil)
		req.RemoteAddr = ip + ":4321"
		req.AddCookie(stale)
		if rec := serve(t, g, req); rec.Code != http.StatusUnauthorized {
			t.Fatalf("失效 cookie 第 %d 次应一直是 401（不是 429）: %d", i+1, rec.Code)
		}
	}
	login := postJSON("/api/v1/auth/login", `{"passcode":"`+testPasscode+`"}`)
	login.RemoteAddr = ip + ":4321"
	if rec := serve(t, g, login); rec.Code != http.StatusOK {
		t.Fatalf("失效 cookie 之后应能正常登录: %d %s", rec.Code, rec.Body)
	}
}

// TestValidCookieSurvivesIPBlock 同一个出口 IP 上有人在猜口令被封时，已登录用户的会话不受牵连。
func TestValidCookieSurvivesIPBlock(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	const ip = "203.0.113.40"
	login := postJSON("/api/v1/auth/login", `{"passcode":"`+testPasscode+`"}`)
	login.RemoteAddr = ip + ":4321"
	cookies := serve(t, g, login).Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("登录应发 cookie")
	}
	failTimes(t, g, ip, maxFailedAttemptsPerWindow+1)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/boards", nil)
	req.RemoteAddr = ip + ":4321"
	for _, c := range cookies {
		req.AddCookie(c)
	}
	if rec := serve(t, g, req); rec.Code != http.StatusOK {
		t.Fatalf("封锁期内带有效 cookie 的请求应放行: %d", rec.Code)
	}
}

// TestStatusIsNotAGuessingOracle status 接口不能成为免限流的猜口令入口：
// 用 Bearer 反复试同样会被计数，超限后返回 429，连正确口令也不例外。
func TestStatusIsNotAGuessingOracle(t *testing.T) {
	g := miniPasscodeEngine(testPasscode, nil)
	const ip = "203.0.113.60"
	for i := 0; i < maxFailedAttemptsPerWindow+1; i++ {
		req := withBearer(httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil), fmt.Sprintf("guess-%d", i), ip)
		rec := serve(t, g, req)
		if rec.Code != http.StatusOK && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("status 的错误 Bearer 应返回 200(未认证) 或 429: %d", rec.Code)
		}
	}
	rec := serve(t, g, withBearer(httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil), testPasscode, ip))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("封锁后 status 带正确口令也应 429: %d", rec.Code)
	}
}
