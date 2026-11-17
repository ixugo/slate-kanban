package kanban

import (
	"errors"
	"testing"
)

// TestNormalizeListLimit 验证列表数量的换算：0 取默认值，正常值原样使用，超过上限取上限，负数报错。
func TestNormalizeListLimit(t *testing.T) {
	cases := []struct {
		in   int
		want int
		bad  bool
	}{
		{0, defaultListLimit, false},
		{1, 1, false},
		{maxListLimit, maxListLimit, false},
		{maxListLimit + 1, maxListLimit, false},
		{1 << 30, maxListLimit, false},
		{-1, 0, true},
	}
	for _, c := range cases {
		got, err := normalizeListLimit(c.in)
		if c.bad {
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("limit=%d：应该得到 ErrInvalidInput，实际 %v", c.in, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("limit=%d：期望 %d，实际 %d, %v", c.in, c.want, got, err)
		}
	}
}
