package kanban_test

import (
	"fmt"
	"testing"

	"slate-kanban/internal/core/kanban"
)

func TestCardUpdatePreviousValues(t *testing.T) {
	ctx := t.Context()
	e := newCardEnv(t)
	engineer, _, _ := e.twoDevOneReview(t)
	card := e.card(t, colTodo, "任务")
	first := e.card(t, colTodo, "前置一")
	second := e.card(t, colTodo, "前置二")
	_, err := e.core.UpdateCard(ctx, testActor, card.ID, kanban.UpdateCardInput{
		Branch: new("main"), URL: new("https://example.com/old"), HumanOnly: new(true),
		Priority: new(3), EngineerID: new(engineer.ID), WaitsOn: new([]int64{second.ID, first.ID}),
	})
	if err != nil {
		t.Fatal(err)
	}
	events := len(e.events(t))
	_, err = e.core.UpdateCard(ctx, testActor, card.ID, kanban.UpdateCardInput{
		Branch: new("dev"), URL: new("https://example.com/new"), HumanOnly: new(false),
		Priority: new(1), EngineerID: new(int64(0)), WaitsOn: new([]int64{first.ID}),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{"fields":["branch","url","priority","human_only","engineer_id","waits_on"],"key":%q,"previous_values":{"branch":"main","engineer_id":"全栈一号","human_only":true,"priority":3,"url":"https://example.com/old","waits_on":[{"id":%d,"number":%d},{"id":%d,"number":%d}]},"title":"任务"}`,
		card.Key, second.ID, second.Number, first.ID, first.Number)
	if ev := e.lastEvent(t); ev.Type != kanban.EventCardUpdated || ev.Payload != want {
		t.Fatalf("修改事件 = %s，期望 %s", ev.Payload, want)
	}
	if got := len(e.events(t)) - events; got != 1 {
		t.Fatalf("此次更新新增 %d 条事件. 期望 1 条", got)
	}
}
