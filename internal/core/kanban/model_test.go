package kanban_test

import (
	"slices"
	"testing"
	"time"

	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
)

// migrateAll 迁移全部领域模型，供模型测试使用。
func migrateAll(t *testing.T) *gorm.DB {
	t.Helper()
	db := kanbantest.NewRawDB(t)
	err := db.AutoMigrate(
		new(kanban.Board), new(kanban.Column), new(kanban.Card), new(kanban.Comment),
		new(kanban.Rule), new(kanban.Event), new(kanban.AgentCursor),
	)
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

// TestModelsAutoMigrateCreatesIndexes 验证迁移后，设计文档里依赖的唯一索引和查询索引都存在。
// 唯一索引保证 slug、卡片 Key、agent 游标不会重复；查询索引保证按看板取卡片、取事件走索引。
func TestModelsAutoMigrateCreatesIndexes(t *testing.T) {
	db := migrateAll(t)

	var names []string
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type = 'index'").Scan(&names).Error; err != nil {
		t.Fatalf("读取索引失败: %v", err)
	}
	want := []string{
		"uq_boards_slug", "uq_cards_key", "uq_agent_cursors_board_agent",
		"idx_board_columns_board_sort", "idx_cards_board_column_pos",
		"idx_comments_card", "idx_comments_board", "idx_rules_board_sort",
		"idx_events_board_seq", "idx_events_card",
	}
	for _, name := range want {
		if !slices.Contains(names, name) {
			t.Errorf("缺少索引 %s，现有索引: %v", name, names)
		}
	}
}

// TestCardRoundTrip 验证卡片的 JSON 字段和可空时间能原样存取：
// 有值的标签、依赖、时间读回来保持不变，没填的时间读回来仍然是 nil。
func TestCardRoundTrip(t *testing.T) {
	db := migrateAll(t)
	due := time.Date(2026, 12, 1, 8, 30, 0, 0, time.UTC)
	in := &kanban.Card{
		BoardID: 1, ColumnID: 2, Number: 1, Key: "SLATE-1", Title: "写设计文档",
		TagIDs: []int64{3, 4}, WaitsOn: []int64{7, 9}, DueAt: &due, Version: 1,
	}
	if err := db.Create(in).Error; err != nil {
		t.Fatalf("创建卡片失败: %v", err)
	}

	var got kanban.Card
	if err := db.First(&got, in.ID).Error; err != nil {
		t.Fatalf("读取卡片失败: %v", err)
	}
	got.Normalize()
	if !slices.Equal(got.TagIDs, in.TagIDs) || !slices.Equal(got.WaitsOn, in.WaitsOn) {
		t.Errorf("JSON 字段不一致: tag_ids=%v waits_on=%v", got.TagIDs, got.WaitsOn)
	}
	if got.DueAt == nil || !got.DueAt.Equal(due) {
		t.Errorf("到期时间不一致: %v", got.DueAt)
	}
	if got.StartedAt != nil || got.FinishedAt != nil || got.ClaimExpiresAt != nil {
		t.Errorf("没填的时间应该保持 nil: %v %v %v", got.StartedAt, got.FinishedAt, got.ClaimExpiresAt)
	}
}

// TestCardNormalizeNilSlices 验证没有标签和依赖的卡片读回来后，经过 Normalize 是空切片而不是 nil，
// 这样序列化出来的 JSON 是 [] 而不是 null。
func TestCardNormalizeNilSlices(t *testing.T) {
	db := migrateAll(t)
	in := &kanban.Card{BoardID: 1, ColumnID: 1, Number: 1, Key: "SLATE-2", Title: "无标签", Version: 1}
	if err := db.Create(in).Error; err != nil {
		t.Fatalf("创建卡片失败: %v", err)
	}
	var got kanban.Card
	if err := db.First(&got, in.ID).Error; err != nil {
		t.Fatalf("读取卡片失败: %v", err)
	}
	got.Normalize()
	if got.TagIDs == nil || got.WaitsOn == nil {
		t.Errorf("Normalize 之后不应该是 nil: tag_ids=%v waits_on=%v", got.TagIDs, got.WaitsOn)
	}
}

// TestCardKeyIsUnique 验证卡片 Key 的唯一索引生效：重复的 Key 必须写入失败。
func TestCardKeyIsUnique(t *testing.T) {
	db := migrateAll(t)
	first := &kanban.Card{BoardID: 1, ColumnID: 1, Number: 1, Key: "SLATE-3", Title: "一", Version: 1}
	if err := db.Create(first).Error; err != nil {
		t.Fatalf("创建第一张卡片失败: %v", err)
	}
	dup := &kanban.Card{BoardID: 2, ColumnID: 1, Number: 1, Key: "SLATE-3", Title: "二", Version: 1}
	if err := db.Create(dup).Error; err == nil {
		t.Fatal("重复的 Key 应该写入失败")
	}
}
