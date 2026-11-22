package kanban_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestSendWebhookBodies 验证三种机器人请求体与钉钉加签参数。
func TestSendWebhookBodies(t *testing.T) {
	var gotURL, gotBody string
	restore := kanban.SetWebhookHTTPDoForTest(func(_ *http.Client, req *http.Request) (*http.Response, error) {
		gotURL = req.URL.String()
		b, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("读 body: %v", err)
		}
		gotBody = string(b)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
	})
	defer restore()

	ctx := context.Background()
	if err := kanban.SendWebhookText(ctx, &kanban.Webhook{
		Type: kanban.WebhookTypeDingTalk, URL: "https://oapi.dingtalk.com/robot/send?access_token=x", Secret: "sec",
	}, "hello"); err != nil {
		t.Fatalf("钉钉: %v", err)
	}
	if !strings.Contains(gotURL, "timestamp=") || !strings.Contains(gotURL, "sign=") {
		t.Fatalf("钉钉 URL 应带加签: %s", gotURL)
	}
	dingURL, err := url.Parse(gotURL)
	if err != nil {
		t.Fatal(err)
	}
	query := dingURL.Query()
	mac := hmac.New(sha256.New, []byte("sec"))
	_, _ = mac.Write([]byte(query.Get("timestamp") + "\nsec"))
	if query.Get("sign") != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatal("钉钉 URL 解码一次后应得到正确的 Base64 签名")
	}
	var ding map[string]any
	if err := json.Unmarshal([]byte(gotBody), &ding); err != nil || ding["msgtype"] != "text" {
		t.Fatalf("钉钉 body 不对: %s", gotBody)
	}

	gotURL = ""
	if err := kanban.SendWebhookText(ctx, &kanban.Webhook{
		Type: kanban.WebhookTypeWeCom, URL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=k",
	}, "hi"); err != nil {
		t.Fatalf("企微: %v", err)
	}
	if gotURL == "" {
		t.Fatal("企微应发请求")
	}

	gotBody = ""
	if err := kanban.SendWebhookText(ctx, &kanban.Webhook{
		Type: kanban.WebhookTypeFeishu, URL: "https://open.feishu.cn/open-apis/bot/v2/hook/x", Secret: "s",
	}, "飞书"); err != nil {
		t.Fatalf("飞书: %v", err)
	}
	var fei map[string]any
	if err := json.Unmarshal([]byte(gotBody), &fei); err != nil || fei["msg_type"] != "text" || fei["sign"] == nil {
		t.Fatalf("飞书 body 应含签名: %s", gotBody)
	}
}

// TestWebhookResponseBizError 三种机器人在 HTTP 200 但业务错误码非零时应失败，且带上平台 errmsg。
func TestWebhookResponseBizError(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		wh   *kanban.Webhook
		resp string
		want string
	}{
		{
			name: "钉钉 token 无效",
			wh:   &kanban.Webhook{Type: kanban.WebhookTypeDingTalk, URL: "https://oapi.dingtalk.com/robot/send?access_token=x"},
			resp: `{"errcode":300001,"errmsg":"token is not exist"}`,
			want: "token is not exist",
		},
		{
			name: "企微 key 无效",
			wh:   &kanban.Webhook{Type: kanban.WebhookTypeWeCom, URL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=k"},
			resp: `{"errcode":93000,"errmsg":"invalid webhook url"}`,
			want: "invalid webhook url",
		},
		{
			name: "飞书 hook 无效",
			wh:   &kanban.Webhook{Type: kanban.WebhookTypeFeishu, URL: "https://open.feishu.cn/open-apis/bot/v2/hook/x"},
			resp: `{"code":9499,"msg":"bad request"}`,
			want: "bad request",
		},
		{
			name: "飞书旧版 StatusCode",
			wh:   &kanban.Webhook{Type: kanban.WebhookTypeFeishu, URL: "https://open.feishu.cn/open-apis/bot/v2/hook/x"},
			resp: `{"StatusCode":19021,"StatusMessage":"sign match fail"}`,
			want: "sign match fail",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			restore := kanban.SetWebhookHTTPDoForTest(func(_ *http.Client, _ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(tc.resp)),
					Header:     make(http.Header),
				}, nil
			})
			defer restore()
			err := kanban.SendWebhookText(ctx, tc.wh, "ping")
			if err == nil {
				t.Fatal("应返回错误")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("错误应含 %q，实际: %v", tc.want, err)
			}
			if strings.Contains(err.Error(), tc.wh.URL) {
				t.Fatalf("错误不应带出完整 URL: %v", err)
			}
		})
	}
}

// TestWebhookResponseBizOK 业务成功体或空 JSON 在 HTTP 200 时应视为发送成功。
func TestWebhookResponseBizOK(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		wh   *kanban.Webhook
		resp string
	}{
		{name: "钉钉 errcode 0", wh: &kanban.Webhook{Type: kanban.WebhookTypeDingTalk, URL: "https://oapi.dingtalk.com/robot/send?access_token=x"}, resp: `{"errcode":0,"errmsg":"ok"}`},
		{name: "企微 errcode 0", wh: &kanban.Webhook{Type: kanban.WebhookTypeWeCom, URL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=k"}, resp: `{"errcode":0,"errmsg":"ok"}`},
		{name: "飞书 code 0", wh: &kanban.Webhook{Type: kanban.WebhookTypeFeishu, URL: "https://open.feishu.cn/open-apis/bot/v2/hook/x"}, resp: `{"code":0,"msg":"success"}`},
		{name: "空对象", wh: &kanban.Webhook{Type: kanban.WebhookTypeWeCom, URL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=k"}, resp: `{}`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			restore := kanban.SetWebhookHTTPDoForTest(func(_ *http.Client, _ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(tc.resp)),
					Header:     make(http.Header),
				}, nil
			})
			defer restore()
			if err := kanban.SendWebhookText(ctx, tc.wh, "ok"); err != nil {
				t.Fatalf("应成功: %v", err)
			}
		})
	}
}

// TestWebhookResponseNonJSON 非 JSON 响应体按 HTTP 状态码判定。
func TestWebhookResponseNonJSON(t *testing.T) {
	ctx := context.Background()
	wh := &kanban.Webhook{Type: kanban.WebhookTypeWeCom, URL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=k"}

	t.Run("200 plain", func(t *testing.T) {
		restore := kanban.SetWebhookHTTPDoForTest(func(_ *http.Client, _ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("OK")), Header: make(http.Header)}, nil
		})
		defer restore()
		if err := kanban.SendWebhookText(ctx, wh, "x"); err != nil {
			t.Fatalf("200 非 JSON 应成功: %v", err)
		}
	})
	t.Run("502", func(t *testing.T) {
		restore := kanban.SetWebhookHTTPDoForTest(func(_ *http.Client, _ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("upstream")), Header: make(http.Header)}, nil
		})
		defer restore()
		err := kanban.SendWebhookText(ctx, wh, "x")
		if err == nil || !strings.Contains(err.Error(), "502") {
			t.Fatalf("非 2xx 应失败: %v", err)
		}
	})
}
