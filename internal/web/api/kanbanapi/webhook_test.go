package kanbanapi_test

import (
	"net/http"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// webhookView 是接口返回里测试关心的字段。
type webhookView struct {
	ID                int64                     `json:"id"`
	Name              string                    `json:"name"`
	Type              string                    `json:"type"`
	SecretSet         bool                      `json:"secret_set"`
	Enabled           bool                      `json:"enabled"`
	StrictFilterModes kanban.WebhookFilterModes `json:"strict_filter_modes"`
	StrictFilters     bool                      `json:"strict_filters"`
}

// TestWebhookAPI 验证 webhook 的 CRUD、脱敏与测试发送。
func TestWebhookAPI(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "WHK")
	base := "/boards/" + id(b.ID) + "/webhooks"

	restore := kanban.SetWebhookHTTPDoForTest(func(_ *http.Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	defer restore()

	made := decode[webhookView](t, s.human(t, http.MethodPost, base, map[string]any{
		"name": "群通知", "type": "wecom",
		"url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=abc", "enabled": true,
	}))
	if made.ID == 0 || made.SecretSet || !made.Enabled || made.StrictFilters {
		t.Fatalf("新增不对: %+v", made)
	}
	list := decode[[]webhookView](t, s.human(t, http.MethodGet, base, nil))
	if len(list) != 1 || list[0].ID != made.ID {
		t.Fatalf("列表: %+v", list)
	}
	path := "/webhooks/" + id(made.ID)
	strict := decode[webhookView](t, s.human(t, http.MethodPatch, path, map[string]any{
		"strict_filters": true, "event_types": []string{}, "actor_kinds": []string{}, "move_column_ids": []int64{},
	}))
	if !strict.StrictFilters {
		t.Fatalf("需要保存显式订阅模式: %+v", strict)
	}
	list = decode[[]webhookView](t, s.human(t, http.MethodGet, base, nil))
	if len(list) != 1 || !list[0].StrictFilters {
		t.Fatalf("订阅模式需从数据库读回: %+v", list)
	}
	override := decode[webhookView](t, s.human(t, http.MethodPatch, path, map[string]any{
		"strict_filter_modes": map[string]bool{"actor_kinds": false},
	}))
	if !override.StrictFilters || override.StrictFilterModes.ActorKinds == nil || *override.StrictFilterModes.ActorKinds ||
		override.StrictFilterModes.EventTypes != nil || override.StrictFilterModes.MoveColumnIDs != nil {
		t.Fatalf("按维覆盖应保留兼容开关和其他筛选维度: %+v", override)
	}
	ding := decode[webhookView](t, s.human(t, http.MethodPost, base, map[string]any{
		"name": "钉钉", "type": "dingtalk",
		"url": "https://oapi.dingtalk.com/robot/send?access_token=abc",
	}))
	dingPath := "/webhooks/" + id(ding.ID)
	withSecret := decode[webhookView](t, s.human(t, http.MethodPatch, dingPath, map[string]any{
		"secret": "top-secret",
	}))
	if !withSecret.SecretSet {
		t.Fatalf("应显示已设置密钥: %+v", withSecret)
	}
	tested := decode[map[string]any](t, s.human(t, http.MethodPost, path+"/test", nil))
	if tested["ok"] != true {
		t.Fatalf("测试发送应成功: %+v", tested)
	}
	wantFail(t, "非法 URL", s.human(t, http.MethodPost, base, map[string]any{
		"name": "坏", "type": "dingtalk", "url": "https://evil.com/x",
	}), http.StatusBadRequest, "ErrInvalidInput")
	wantEmptyObject(t, "删除", s.human(t, http.MethodDelete, path, nil))
}
