package kanban

import (
	"errors"
	"testing"
)

// cardsAt 按给定的位置构造一列卡片，主键依次是 1、2、3……。
func cardsAt(positions ...float64) []*Card {
	cards := make([]*Card, len(positions))
	for i, p := range positions {
		cards[i] = &Card{ID: int64(i + 1), Position: p}
	}
	return cards
}

// TestComputePosition 验证位置计算：空列、追加到列尾、插到列头、插到两张卡中间、插到最后一张后面。
func TestComputePosition(t *testing.T) {
	cases := []struct {
		name          string
		cards         []*Card
		before, after int64
		want          float64
		wantIdx       int
	}{
		{"空列", nil, 0, 0, 1024, 0},
		{"追加到列尾", cardsAt(1024, 2048), 0, 0, 3072, 2},
		{"放到第一张前面", cardsAt(1024, 2048), 1, 0, 0, 0},
		{"放到第二张前面", cardsAt(1024, 2048), 2, 0, 1536, 1},
		{"放到第一张后面", cardsAt(1024, 2048), 0, 1, 1536, 1},
		{"放到最后一张后面", cardsAt(1024, 2048), 0, 2, 3072, 2},
		{"列头可以出现负数", cardsAt(0), 1, 0, -1024, 0},
		{"中点保留小数", cardsAt(1, 2), 0, 1, 1.5, 1},
	}
	for _, c := range cases {
		got, idx, err := computePosition(c.cards, c.before, c.after)
		if err != nil || got != c.want || idx != c.wantIdx {
			t.Errorf("%s：期望位置 %v 下标 %d，实际 %v 下标 %d, %v", c.name, c.want, c.wantIdx, got, idx, err)
		}
	}
}

// TestComputePositionErrors 验证不合法的请求：两个参照同时给、参照卡片不在这一列里。
func TestComputePositionErrors(t *testing.T) {
	cards := cardsAt(1024, 2048)
	for _, c := range []struct{ before, after int64 }{{1, 2}, {99, 0}, {0, 99}} {
		if _, _, err := computePosition(cards, c.before, c.after); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("before=%d after=%d 应该得到 ErrInvalidInput，实际 %v", c.before, c.after, err)
		}
	}
}

// TestComputePositionNeedsRenumber 验证相邻位置太近、或者位置相同的时候要求整列重排，
// 而间隙刚好够用时仍然可以直接插入。
func TestComputePositionNeedsRenumber(t *testing.T) {
	for _, cards := range [][]*Card{cardsAt(1, 1+1e-7), cardsAt(5, 5), cardsAt(1, 1+1.5e-6)} {
		if _, _, err := computePosition(cards, 0, 1); !errors.Is(err, errNeedRenumber) {
			t.Errorf("%v 应该要求整列重排，实际 %v", []float64{cards[0].Position, cards[1].Position}, err)
		}
	}
	cards := cardsAt(1, 1+4e-6)
	got, _, err := computePosition(cards, 0, 1)
	if err != nil || got <= cards[0].Position || got >= cards[1].Position {
		t.Fatalf("间隙够用时应该直接取中点，实际 %v, %v", got, err)
	}
}
