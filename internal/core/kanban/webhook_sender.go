package kanban

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	webhookHTTPTimeout      = 10 * time.Second
	webhookMaxResponseBytes = 64 * 1024
	webhookMaxRedirects     = 5
)

// webhookHTTPDo 是实际发 HTTP 请求的函数，测试里可替换。
var webhookHTTPDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
	return client.Do(req)
}

// SetWebhookHTTPDoForTest 让测试接管 HTTP 发送。
func SetWebhookHTTPDoForTest(fn func(*http.Client, *http.Request) (*http.Response, error)) func() {
	prev := webhookHTTPDo
	webhookHTTPDo = fn
	return func() { webhookHTTPDo = prev }
}

// newWebhookHTTPClient 创建带超时、响应体上限与同主机跳转限制的客户端。
func newWebhookHTTPClient() *http.Client {
	return &http.Client{
		Timeout: webhookHTTPTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= webhookMaxRedirects {
				return fmt.Errorf("webhook 跳转次数过多")
			}
			if len(via) > 0 && req.URL.Hostname() != via[0].URL.Hostname() {
				return fmt.Errorf("webhook 不允许跳转到其他主机")
			}
			return nil
		},
	}
}

// SendWebhookText 按类型把文本消息发到群机器人；失败返回可读错误，不写日志。
func SendWebhookText(ctx context.Context, w *Webhook, text string) error {
	switch w.Type {
	case WebhookTypeDingTalk:
		return sendDingTalk(ctx, w, text)
	case WebhookTypeWeCom:
		return sendWeCom(ctx, w, text)
	case WebhookTypeFeishu:
		return sendFeishu(ctx, w, text)
	default:
		return invalidf("不认识的 webhook 类型 %q", w.Type)
	}
}

// sendDingTalk 发送钉钉文本消息，可选加签。
func sendDingTalk(ctx context.Context, w *Webhook, text string) error {
	target, err := dingTalkURL(w)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": text},
	})
	if err != nil {
		return reasonJSON(err)
	}
	return postWebhookJSON(ctx, WebhookTypeDingTalk, target, body, nil)
}

// dingTalkURL 在原始地址上追加钉钉加签参数。
func dingTalkURL(w *Webhook) (string, error) {
	if w.Secret == "" {
		return w.URL, nil
	}
	ts := time.Now().UnixMilli()
	stringToSign := fmt.Sprintf("%d\n%s", ts, w.Secret)
	mac := hmac.New(sha256.New, []byte(w.Secret))
	if _, err := mac.Write([]byte(stringToSign)); err != nil {
		return "", reasonJSON(err)
	}
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	u, err := url.Parse(w.URL)
	if err != nil {
		return "", invalidf("webhook 地址不合法")
	}
	q := u.Query()
	q.Set("timestamp", strconv.FormatInt(ts, 10))
	q.Set("sign", sign)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// sendWeCom 发送企业微信文本消息。
func sendWeCom(ctx context.Context, w *Webhook, text string) error {
	body, err := json.Marshal(map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": text},
	})
	if err != nil {
		return reasonJSON(err)
	}
	return postWebhookJSON(ctx, WebhookTypeWeCom, w.URL, body, nil)
}

// sendFeishu 发送飞书文本消息，可选签名写在 JSON 里。
func sendFeishu(ctx context.Context, w *Webhook, text string) error {
	msg := map[string]any{
		"msg_type": "text",
		"content":  map[string]string{"text": text},
	}
	if w.Secret != "" {
		ts := time.Now().Unix()
		sign, err := feishuSign(ts, w.Secret)
		if err != nil {
			return err
		}
		msg["timestamp"] = strconv.FormatInt(ts, 10)
		msg["sign"] = sign
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return reasonJSON(err)
	}
	return postWebhookJSON(ctx, WebhookTypeFeishu, w.URL, body, nil)
}

// feishuSign 按飞书官方算法生成签名。
func feishuSign(timestamp int64, secret string) (string, error) {
	stringToSign := fmt.Sprintf("%v", timestamp) + "\n" + secret
	h := hmac.New(sha256.New, []byte(stringToSign))
	if _, err := h.Write(nil); err != nil {
		return "", reasonJSON(err)
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// postWebhookJSON 发送 POST JSON，先查 HTTP 状态，再按机器人类型读响应体里的业务错误码。
func postWebhookJSON(ctx context.Context, webhookType, url string, body []byte, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return invalidf("构造 webhook 请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := newWebhookHTTPClient()
	resp, err := webhookHTTPDo(client, req)
	if err != nil {
		return fmt.Errorf("webhook 请求失败: %w", err)
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, webhookMaxResponseBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("读取 webhook 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回 %d: %s", resp.StatusCode, truncateWebhookErrText(string(raw)))
	}
	return webhookBizResultError(webhookType, raw)
}

const maxWebhookErrTextLen = 200

// truncateWebhookErrText 截断并去掉响应片段里可能出现的完整 URL，避免排障日志带出密钥。
func truncateWebhookErrText(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "http://"); i >= 0 {
		s = strings.TrimSpace(s[:i]) + " [URL已省略]"
	}
	if i := strings.Index(s, "https://"); i >= 0 {
		s = strings.TrimSpace(s[:i]) + " [URL已省略]"
	}
	if len(s) > maxWebhookErrTextLen {
		return s[:maxWebhookErrTextLen] + "…"
	}
	return s
}

// webhookBizResultError 解析 2xx 响应体里的 errcode/code，钉钉与企微格式相同，飞书另有旧版字段。
func webhookBizResultError(webhookType string, raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	switch webhookType {
	case WebhookTypeDingTalk, WebhookTypeWeCom:
		if code, ok := webhookJSONInt(m, "errcode"); ok && code != 0 {
			msg := truncateWebhookErrText(webhookJSONString(m, "errmsg"))
			if msg == "" {
				msg = fmt.Sprintf("错误码 %d", code)
			}
			return fmt.Errorf("webhook 接口拒绝: %s", msg)
		}
	case WebhookTypeFeishu:
		if code, ok := webhookJSONInt(m, "code"); ok && code != 0 {
			msg := truncateWebhookErrText(webhookJSONString(m, "msg"))
			if msg == "" {
				msg = fmt.Sprintf("错误码 %d", code)
			}
			return fmt.Errorf("webhook 接口拒绝: %s", msg)
		}
		if code, ok := webhookJSONInt(m, "StatusCode"); ok && code != 0 {
			msg := truncateWebhookErrText(webhookJSONString(m, "StatusMessage"))
			if msg == "" {
				msg = fmt.Sprintf("错误码 %d", code)
			}
			return fmt.Errorf("webhook 接口拒绝: %s", msg)
		}
	}
	return nil
}

// webhookJSONInt 从 JSON 对象读整数，兼容 float64 与整型。
func webhookJSONInt(m map[string]any, key string) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

// webhookJSONString 读字符串字段，非字符串则格式化成简短文本。
func webhookJSONString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// reasonJSON 把 JSON 编码错误包成领域可识别的错误。
func reasonJSON(err error) error {
	return ErrInvalidInput.WithCause(err).WithMsg("编码 webhook 消息失败")
}
