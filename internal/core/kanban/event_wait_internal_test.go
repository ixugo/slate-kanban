package kanban

import (
	"errors"
	"testing"
	"time"
)

// TestNormalizeWaitTimeout 验证等待时间的换算：0 取默认值，正常值原样使用，超过上限取上限，负数报错。
func TestNormalizeWaitTimeout(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want time.Duration
		bad  bool
	}{
		{0, defaultWaitTimeout, false},
		{time.Millisecond, time.Millisecond, false},
		{50 * time.Second, 50 * time.Second, false},
		{MaxWaitTimeout, MaxWaitTimeout, false},
		{MaxWaitTimeout + time.Second, MaxWaitTimeout, false},
		{time.Hour, time.Hour, false},
		{-time.Second, 0, true},
	}
	for _, c := range cases {
		got, err := normalizeWaitTimeout(c.in)
		if c.bad {
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("timeout=%v：应该得到 ErrInvalidInput，实际 %v", c.in, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("timeout=%v：期望 %v，实际 %v, %v", c.in, c.want, got, err)
		}
	}
}
