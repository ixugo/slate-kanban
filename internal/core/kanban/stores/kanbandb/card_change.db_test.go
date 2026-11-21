package kanbandb_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// addCardChange 写入一条测试用的描述变更记录并返回。
func addCardChange(t *testing.T, store kanban.CardChangeStorer, boardID, cardID int64, oldValue, newValue string) *kanban.CardChange {
	t.Helper()
	c := &kanban.CardChange{
		BoardID: boardID, CardID: cardID, Field: kanban.CardFieldDescription,
		OldValue: oldValue, NewValue: newValue, ActorKind: "human", Actor: "admin",
	}
	if err := store.Create(context.Background(), c); err != nil {
		t.Fatalf("创建变更记录失败: %v", err)
	}
	return c
}

// TestCardChangeCreateGet 验证变更记录能按主键原样读回，主键递增，不存在的主键是找不到记录。
func TestCardChangeCreateGet(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.CardChange()
		first := addCardChange(t, store, 1, 10, "旧", "新")
		second := addCardChange(t, store, 1, 10, "新", "更新")
		if first.ID == 0 || second.ID <= first.ID {
			t.Fatalf("主键应该递增: %d %d", first.ID, second.ID)
		}

		got, err := store.GetByID(ctx, first.ID)
		if err != nil {
			t.Fatalf("按主键读取失败: %v", err)
		}
		if got.BoardID != 1 || got.CardID != 10 || got.Field != "description" || got.OldValue != "旧" || got.NewValue != "新" ||
			got.ActorKind != "human" || got.Actor != "admin" || got.CreatedAt.IsZero() {
			t.Fatalf("读回的记录不对: %+v", got)
		}
		if _, err := store.GetByID(ctx, 9999); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("不存在的主键应该是找不到记录，实际: %v", err)
		}
	})
}

// TestCardChangeKeepsLongMultilineText 验证描述上限（两万字）的多行中文、符号和表情都能原样存取，
// 换行、首尾空白和 emoji 一个字符都不丢，两种数据库行为一致。
func TestCardChangeKeepsLongMultilineText(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.CardChange()
		oldText := "  # 标题\n\n" + strings.Repeat("第一行：中文，English，😀 \"引号\" 'single' \\ 反斜杠\n", 350) + "尾部空白  \n\n"
		newText := strings.Repeat("新内容\t含制表符\r\n", 1000)

		saved := addCardChange(t, store, 1, 10, oldText, newText)
		got, err := store.GetByID(ctx, saved.ID)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if got.OldValue != oldText || got.NewValue != newText {
			t.Fatalf("长文本没有原样读回: 旧 %d/%d 字节，新 %d/%d 字节", len(got.OldValue), len(oldText), len(got.NewValue), len(newText))
		}
	})
}

// TestCardChangeDelete 验证按卡片、按看板删除都只影响自己的范围。
func TestCardChangeDelete(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.CardChange()
		a := addCardChange(t, store, 1, 10, "a1", "a2")
		b := addCardChange(t, store, 1, 11, "b1", "b2")
		c := addCardChange(t, store, 2, 20, "c1", "c2")

		if err := store.DeleteByCard(ctx, 10); err != nil {
			t.Fatalf("按卡片删除失败: %v", err)
		}
		if _, err := store.GetByID(ctx, a.ID); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("卡片 10 的记录应该没了: %v", err)
		}
		if _, err := store.GetByID(ctx, b.ID); err != nil {
			t.Fatalf("卡片 11 的记录不该受影响: %v", err)
		}
		if err := store.DeleteByBoard(ctx, 1); err != nil {
			t.Fatalf("按看板删除失败: %v", err)
		}
		if _, err := store.GetByID(ctx, b.ID); !orm.IsErrRecordNotFound(err) {
			t.Fatalf("看板 1 的记录应该没了: %v", err)
		}
		if _, err := store.GetByID(ctx, c.ID); err != nil {
			t.Fatalf("看板 2 的记录不该受影响: %v", err)
		}
		// 没有可删的记录也算成功
		if err := store.DeleteByCard(ctx, 999); err != nil {
			t.Fatalf("删除不存在的卡片的记录应该成功: %v", err)
		}
	})
}
