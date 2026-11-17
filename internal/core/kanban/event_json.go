package kanban

import (
	"bytes"
	"encoding/json"
)

// MarshalJSON 把事件序列化成 JSON，和默认行为只有一处不同：Payload 在数据库里是存成文本的 JSON，
// 这里直接输出成 JSON 对象，而不是再套一层转义字符串，网页和 agent 拿到就能直接读。
// 用别名类型去掉方法再内嵌，是为了不用把 Event 的字段重复抄一遍，以后给 Event 加字段也不会漏。
// 存的内容万一不是合法 JSON（正常不会发生），退回成字符串输出，保证整个响应不会因为一条事件而坏掉。
func (e Event) MarshalJSON() ([]byte, error) {
	type plain Event
	raw := json.RawMessage("{}")
	switch {
	case e.Payload == "":
	case json.Valid([]byte(e.Payload)):
		raw = json.RawMessage(e.Payload)
	default:
		quoted, err := json.Marshal(e.Payload)
		if err != nil {
			return nil, err
		}
		raw = quoted
	}
	return json.Marshal(struct {
		plain
		Payload json.RawMessage `json:"payload"`
	}{plain: plain(e), Payload: raw})
}

// UnmarshalJSON 是 MarshalJSON 的逆操作：payload 是对象时存成紧凑的 JSON 文本；
// 也兼容旧写法，也就是 payload 是一个装着 JSON 文本的字符串，缺失或 null 时存成空串。
func (e *Event) UnmarshalJSON(data []byte) error {
	type plain Event
	aux := struct {
		*plain
		Payload json.RawMessage `json:"payload"`
	}{plain: (*plain)(e)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	raw := bytes.TrimSpace(aux.Payload)
	switch {
	case len(raw) == 0 || bytes.Equal(raw, []byte("null")):
		e.Payload = ""
	case raw[0] == '"':
		return json.Unmarshal(raw, &e.Payload)
	default:
		var buf bytes.Buffer
		if err := json.Compact(&buf, raw); err != nil {
			return err
		}
		e.Payload = buf.String()
	}
	return nil
}
