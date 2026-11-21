package kanban

import (
	"net/url"
	"slices"
	"strings"
)

// webhookHosts 记录各类型允许的 webhook 主机名，测试可通过 SetWebhookHostsForTest 替换。
var webhookHosts = map[string][]string{
	WebhookTypeDingTalk: {"oapi.dingtalk.com"},
	WebhookTypeWeCom:    {"qyapi.weixin.qq.com"},
	WebhookTypeFeishu:   {"open.feishu.cn", "open.larksuite.com"},
}

// SetWebhookHostsForTest 在测试里临时替换主机白名单，返回恢复函数。
func SetWebhookHostsForTest(hosts map[string][]string) func() {
	prev := webhookHosts
	webhookHosts = hosts
	return func() { webhookHosts = prev }
}

// ValidateWebhookURLForTest 供测试直接调用 URL 校验。
func ValidateWebhookURLForTest(typ, raw string) error {
	return validateWebhookURL(typ, raw)
}

// NormalizeWebhookSubscriptionsForTest 供测试直接调用订阅校验。
func NormalizeWebhookSubscriptionsForTest(w *Webhook, cols []*Column) error {
	return normalizeWebhookSubscriptions(w, cols)
}

// validateWebhookType 检查 webhook 类型是否认识。
func validateWebhookType(typ string) error {
	switch typ {
	case WebhookTypeDingTalk, WebhookTypeWeCom, WebhookTypeFeishu:
		return nil
	default:
		return invalidf("webhook 类型 %q 不认识，可选 dingtalk、wecom、feishu", typ)
	}
}

// validateWebhookURL 只允许 https，并按类型校验主机白名单。
func validateWebhookURL(typ, raw string) error {
	raw = strings.TrimSpace(raw)
	if err := checkLine("webhook 地址", raw, 1, MaxWebhookURLLen); err != nil {
		return err
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return invalidf("webhook 地址必须是合法的 https URL")
	}
	host := strings.ToLower(u.Hostname())
	allowed := webhookHosts[typ]
	if len(allowed) == 0 {
		return invalidf("webhook 类型 %q 未配置主机白名单", typ)
	}
	if !slices.Contains(allowed, host) {
		return invalidf("webhook 地址的主机 %q 不在 %s 类型允许的范围", host, typ)
	}
	return nil
}

// validateWebhookSecret 校验可选密钥长度；钉钉和飞书才需要密钥，企业微信应留空。
func validateWebhookSecret(typ, secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil
	}
	if typ == WebhookTypeWeCom {
		return invalidf("企业微信 webhook 不支持签名密钥")
	}
	if err := checkLine("签名密钥", secret, 1, MaxWebhookSecretLen); err != nil {
		return err
	}
	return nil
}

// normalizeWebhook 整理名称、类型、地址和密钥，并校验订阅条件。
func normalizeWebhook(w *Webhook, cols []*Column) error {
	w.Name = strings.TrimSpace(w.Name)
	if err := checkLine("webhook 名称", w.Name, 1, MaxWebhookNameLen); err != nil {
		return err
	}
	w.Type = strings.TrimSpace(w.Type)
	if err := validateWebhookType(w.Type); err != nil {
		return err
	}
	w.URL = strings.TrimSpace(w.URL)
	if err := validateWebhookURL(w.Type, w.URL); err != nil {
		return err
	}
	w.Secret = strings.TrimSpace(w.Secret)
	if err := validateWebhookSecret(w.Type, w.Secret); err != nil {
		return err
	}
	return normalizeWebhookSubscriptions(w, cols)
}

// normalizeWebhookSubscriptions 校验并去重订阅字段；空切片表示订阅全部。
func normalizeWebhookSubscriptions(w *Webhook, cols []*Column) error {
	w.Normalize()
	types, err := normalizeEventTypes(w.EventTypes)
	if err != nil {
		return err
	}
	if types == nil {
		types = []string{}
	}
	w.EventTypes = types
	outKinds, err := normalizeActorKinds(w.ActorKinds)
	if err != nil {
		return err
	}
	w.ActorKinds = outKinds
	outCols, err := normalizeMoveColumnIDs(w.MoveColumnIDs, cols)
	if err != nil {
		return err
	}
	w.MoveColumnIDs = outCols
	return nil
}

// normalizeActorKinds 校验操作者类型并去重。
func normalizeActorKinds(in []string) ([]string, error) {
	var out []string
	for _, k := range in {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		switch ActorKind(k) {
		case ActorHuman, ActorAgent, ActorSystem:
		default:
			return nil, invalidf("不认识的操作者类型 %q", k)
		}
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

// normalizeMoveColumnIDs 校验目标列属于看板并去重。
func normalizeMoveColumnIDs(in []int64, cols []*Column) ([]int64, error) {
	var out []int64
	for _, id := range in {
		if id == 0 {
			continue
		}
		if columnByID(cols, id) == nil {
			return nil, invalidf("订阅的 card.moved 目标列不存在或不属于这个看板")
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	if out == nil {
		out = []int64{}
	}
	return out, nil
}
