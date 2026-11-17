package kanbanapi_test

import (
	"net/http"
	"testing"
)

// claimed 在领活列里放一张卡片，返回看板和卡片，供领取相关的测试使用。
func claimed(t *testing.T, s *stack, title string) (board, card) {
	t.Helper()
	b := s.newBoard(t, "CLM")
	return b, s.newCard(t, b, b.pickup(t).ID, title)
}

// TestClaimAndFinish 验证领取到完成交接的完整流程：不带请求体的领取能用默认租约；
// 别的 agent 和人都领不了；持有者完成交接后卡片移到下一列、领取清除、总结成为评论。
func TestClaimAndFinish(t *testing.T) {
	s := newStack(t)
	b, c := claimed(t, s, "做点事")
	path := "/cards/" + id(c.ID)

	got := decode[cardDetail](t, s.agent(t, "claude", http.MethodPost, path+"/claim", nil))
	if got.ClaimedBy != "claude" || !got.ClaimActive {
		t.Fatalf("应该被 claude 领取: %+v", got)
	}
	if renewed := decode[cardDetail](t, s.agent(t, "claude", http.MethodPost, path+"/claim", map[string]int{"lease_minutes": 30})); !renewed.ClaimActive {
		t.Fatalf("持有者再次领取应该是续租: %+v", renewed)
	}
	wantFail(t, "别的 agent 领取", s.agent(t, "codex", http.MethodPost, path+"/claim", nil), http.StatusConflict, "ErrCardClaimed")
	wantFail(t, "人领取", s.human(t, http.MethodPost, path+"/claim", nil), http.StatusForbidden, "ErrAgentOnly")

	wantFail(t, "总结为空", s.agent(t, "claude", http.MethodPost, path+"/finish", map[string]string{"summary": " "}), http.StatusBadRequest, "ErrInvalidInput")
	done := decode[cardDetail](t, s.agent(t, "claude", http.MethodPost, path+"/finish",
		map[string]string{"summary": "做完了", "branch": "feat/x", "url": "https://example.com/pr/1"}))
	if done.ColumnID == b.pickup(t).ID || done.ClaimedBy != "" || done.ClaimActive {
		t.Fatalf("完成后应该离开领活列并清除领取: %+v", done)
	}
	if len(done.Comments) != 1 || done.Comments[0].Body != "做完了" || done.Comments[0].Author != "claude" {
		t.Fatalf("总结应该成为评论: %+v", done.Comments)
	}
}

// TestReleaseCard 验证释放：持有者自己释放返回成功；人可以收回；释放幂等；
// 被收回之后原持有者再写这张卡会得到 409，知道自己已经不持有了。
func TestReleaseCard(t *testing.T) {
	s := newStack(t)
	_, c := claimed(t, s, "被收回")
	path := "/cards/" + id(c.ID)

	decode[cardDetail](t, s.agent(t, "claude", http.MethodPost, path+"/claim", nil))
	wantEmptyObject(t, "人收回", s.human(t, http.MethodPost, path+"/release", nil))
	wantEmptyObject(t, "重复释放", s.agent(t, "claude", http.MethodPost, path+"/release", nil))
	wantFail(t, "被收回后写卡片", s.agent(t, "claude", http.MethodPatch, path, map[string]string{"title": "还想改"}), http.StatusConflict, "ErrNotHolder")
	if got := decode[cardDetail](t, s.human(t, http.MethodGet, path, nil)); got.ClaimedBy != "" {
		t.Fatalf("领取应该已经清除: %+v", got)
	}
}

// TestNextCard 验证取下一张：agent 取到并领取领活列最前面的卡片；人不能取；
// 没有可做的卡片时返回 {"card": null} 而不是错误。
func TestNextCard(t *testing.T) {
	s := newStack(t)
	b, first := claimed(t, s, "先做这个")
	s.newCard(t, b, b.pickup(t).ID, "再做这个")
	path := "/boards/" + id(b.ID) + "/cards/next"

	wantFail(t, "人取卡", s.human(t, http.MethodPost, path, nil), http.StatusForbidden, "ErrAgentOnly")
	got := decode[struct {
		Card *cardDetail `json:"card"`
	}](t, s.agent(t, "claude", http.MethodPost, path, nil))
	if got.Card == nil || got.Card.ID != first.ID || got.Card.ClaimedBy != "claude" {
		t.Fatalf("应该领到最前面的卡片: %+v", got.Card)
	}

	other := decode[struct {
		Card *cardDetail `json:"card"`
	}](t, s.agent(t, "codex", http.MethodPost, path, nil))
	if other.Card == nil || other.Card.ID == first.ID {
		t.Fatalf("另一个 agent 应该领到第二张: %+v", other.Card)
	}

	third := s.agent(t, "reviewer", http.MethodPost, path, nil)
	if third.Code != http.StatusOK || string(third.Body) != `{"card":null}` {
		t.Fatalf("没有可做的卡片应该返回 {\"card\":null}: %d %s", third.Code, third.Body)
	}
}
