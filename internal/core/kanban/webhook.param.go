package kanban

// CreateWebhookInput 是创建 webhook 的入参。
type CreateWebhookInput struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	URL           string   `json:"url"`
	Secret        string   `json:"secret"`
	Enabled       *bool    `json:"enabled"`
	EventTypes    []string `json:"event_types"`
	ActorKinds    []string `json:"actor_kinds"`
	MoveColumnIDs []int64  `json:"move_column_ids"`
}

// UpdateWebhookInput 是修改 webhook 的入参，只更新出现的字段；secret 传空字符串表示清空。
type UpdateWebhookInput struct {
	Name          *string   `json:"name"`
	Type          *string   `json:"type"`
	URL           *string   `json:"url"`
	Secret        *string   `json:"secret"`
	SecretClear   bool      `json:"secret_clear"`
	Enabled       *bool     `json:"enabled"`
	EventTypes    *[]string `json:"event_types"`
	ActorKinds    *[]string `json:"actor_kinds"`
	MoveColumnIDs *[]int64  `json:"move_column_ids"`
}

// apply 把入参里出现的字段写到 webhook 上。
func (in UpdateWebhookInput) apply(w *Webhook) {
	if in.Name != nil {
		w.Name = *in.Name
	}
	if in.Type != nil {
		w.Type = *in.Type
	}
	if in.URL != nil {
		w.URL = *in.URL
	}
	if in.SecretClear {
		w.Secret = ""
	} else if in.Secret != nil {
		w.Secret = *in.Secret
	}
	if in.Enabled != nil {
		w.Enabled = *in.Enabled
	}
	if in.EventTypes != nil {
		w.EventTypes = append([]string(nil), (*in.EventTypes)...)
	}
	if in.ActorKinds != nil {
		w.ActorKinds = append([]string(nil), (*in.ActorKinds)...)
	}
	if in.MoveColumnIDs != nil {
		w.MoveColumnIDs = append([]int64(nil), (*in.MoveColumnIDs)...)
	}
}

// TestWebhookResult 是测试发送的返回。
type TestWebhookResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}
