package kanbanapi_test

import (
	"net/http"
	"testing"
)

func claimed(t *testing.T, s *stack, title string) (board, card) {
	t.Helper()
	b := s.newBoard(t, "CLM")
	return b, s.newCard(t, b, b.pickup(t).ID, title)
}

// TestClaimAgentOnlyREST 验证 REST 固定人身份：领取、next_card 等人不能用，与 Core 规则一致。
func TestClaimAgentOnlyREST(t *testing.T) {
	s := newStack(t)
	b, c := claimed(t, s, "做点事")
	path := "/cards/" + id(c.ID)
	wantFail(t, "人领取", s.human(t, http.MethodPost, path+"/claim", nil), http.StatusForbidden, "ErrAgentOnly")
	wantFail(t, "人 next_card", s.human(t, http.MethodPost, "/boards/"+id(b.ID)+"/cards/next", nil), http.StatusForbidden, "ErrAgentOnly")
}

// TestReleaseCard 验证释放：人可以把任务从领取状态收回。
func TestReleaseCard(t *testing.T) {
	s := newStack(t)
	_, c := claimed(t, s, "被收回")
	path := "/cards/" + id(c.ID)
	wantEmptyObject(t, "重复释放", s.human(t, http.MethodPost, path+"/release", nil))
	if got := decode[cardDetail](t, s.human(t, http.MethodGet, path, nil)); got.ClaimedBy != "" {
		t.Fatalf("领取应该已经清除: %+v", got)
	}
}
