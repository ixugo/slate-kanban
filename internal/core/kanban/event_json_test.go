package kanban_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
)

// sampleEvent 构造一条带对象内容的事件。
func sampleEvent(payload string) kanban.Event {
	return kanban.Event{
		ID: 7, BoardID: 2, CardID: 3, Type: kanban.EventCardMoved, ActorKind: "agent", Actor: "claude",
		Payload: payload, CreatedAt: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC),
	}
}

// TestEventMarshalPayloadAsObject 验证序列化：内容直接是 JSON 对象，其他字段原样，字段名没有变。
func TestEventMarshalPayloadAsObject(t *testing.T) {
	raw, err := json.Marshal(sampleEvent(`{"from":"待办","to":"进行中"}`))
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	payload, ok := got["payload"].(map[string]any)
	if !ok || payload["to"] != "进行中" || payload["from"] != "待办" {
		t.Fatalf("payload 应该是对象: %s", raw)
	}
	if got["id"] != float64(7) || got["board_id"] != float64(2) || got["card_id"] != float64(3) ||
		got["type"] != kanban.EventCardMoved || got["actor_kind"] != "agent" || got["actor"] != "claude" ||
		got["created_at"] != "2026-11-17T12:00:00Z" {
		t.Fatalf("其他字段不对: %s", raw)
	}
	if len(got) != 8 {
		t.Fatalf("字段数量应该是 8，实际 %d: %s", len(got), raw)
	}
}

// TestEventMarshalOddPayload 验证特殊内容：空内容输出空对象；不是合法 JSON 的内容输出成字符串，
// 整条事件仍然是合法 JSON。
func TestEventMarshalOddPayload(t *testing.T) {
	for payload, want := range map[string]any{"": map[string]any{}, "不是 JSON {": "不是 JSON {"} {
		raw, err := json.Marshal(sampleEvent(payload))
		if err != nil {
			t.Fatalf("序列化 %q 失败: %v", payload, err)
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("%q 的输出不是合法 JSON: %s", payload, raw)
		}
		wantJSON, _ := json.Marshal(want)
		gotJSON, _ := json.Marshal(got["payload"])
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%q 应该输出 %s，实际 %s", payload, wantJSON, gotJSON)
		}
	}
}

// TestEventUnmarshal 验证反序列化：对象存成紧凑文本，和序列化来回一致；
// 旧写法（字符串）、缺失、null 都能读；payload 的类型不对不会静默吞掉。
func TestEventUnmarshal(t *testing.T) {
	orig := sampleEvent(`{"from":"待办","to":"进行中"}`)
	raw, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var back kanban.Event
	if err := json.Unmarshal(raw, &back); err != nil || back != orig {
		t.Fatalf("来回应该一致: %+v %v", back, err)
	}

	cases := map[string]string{
		`{"id":1,"payload":"{\"a\":1}"}`: `{"a":1}`,
		`{"id":1,"payload":{"a": 1}}`:    `{"a":1}`,
		`{"id":1}`:                       "",
		`{"id":1,"payload":null}`:        "",
	}
	for in, want := range cases {
		var ev kanban.Event
		if err := json.Unmarshal([]byte(in), &ev); err != nil || ev.Payload != want || ev.ID != 1 {
			t.Errorf("%s 应该读成 %q: %+v %v", in, want, ev, err)
		}
	}
	var bad kanban.Event
	if err := json.Unmarshal([]byte(`{"id":1,"payload":{`), &bad); err == nil {
		t.Error("残缺的 JSON 应该报错")
	}
	if err := json.Unmarshal([]byte(`{"id":"x"}`), &bad); err == nil {
		t.Error("字段类型不对应该报错")
	}
}

// TestUpdatesPayloadIsObject 验证 agent 拉取到的整体结果里，事件内容也是对象，
// 不是转义字符串，也就是说所有输出事件的地方都走同一个序列化。
func TestUpdatesPayloadIsObject(t *testing.T) {
	e := newCardEnv(t)
	ctx := context.Background()
	if _, err := e.core.PullUpdates(ctx, testAgent, e.board.ID, kanban.PullInput{}); err != nil {
		t.Fatalf("第一次拉取失败: %v", err)
	}
	e.card(t, colTodo, "A")
	updates, err := e.core.PullUpdates(ctx, testAgent, e.board.ID, kanban.PullInput{})
	if err != nil || len(updates.Events) == 0 {
		t.Fatalf("拉取失败或没有事件: %+v %v", updates, err)
	}
	raw, err := json.Marshal(updates)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var out struct {
		Events []struct {
			Payload map[string]any `json:"payload"`
		} `json:"events"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Events) != len(updates.Events) || out.Events[0].Payload == nil {
		t.Fatalf("事件内容应该是对象: %s %v", raw, err)
	}
}
