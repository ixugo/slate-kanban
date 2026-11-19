package kanbandb_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// 并发领取测试的 goroutine 数量，要大到足以在 PostgreSQL 上稳定撞到行锁竞争。
const claimConcurrentWorkers = 12

// 并发自增编号测试的 goroutine 数量。
const incrSeqConcurrentWorkers = 24

// 领取测试用的固定时钟，排除时间漂移带来的租约误判。
var pgClaimNow = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

// TestPostgresJSONRoundTripSpecialChars 验证标签、依赖 JSON 能往返中文与引号，两种后端行为一致。
func TestPostgresJSONRoundTripSpecialChars(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		ctx := context.Background()
		store := db.Card()
		c := &kanban.Card{
			BoardID: 1, ColumnID: 1, Key: "PG-JSON-1", Title: `标题含 "引号"`, Version: 1,
			TagIDs:  []int64{3, 1, 2},
			WaitsOn: []int64{99, 100},
		}
		if err := store.Create(ctx, c); err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		got, err := store.GetByID(ctx, c.ID)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if !slices.Equal(got.TagIDs, c.TagIDs) || !slices.Equal(got.WaitsOn, c.WaitsOn) {
			t.Fatalf("JSON 往返不一致: tag_ids=%v waits_on=%v", got.TagIDs, got.WaitsOn)
		}
	})
}

// TestPostgresConcurrentIncrCardSeq 验证并发自增卡片编号不出现重复或空洞（依赖原子 UPDATE）。
func TestPostgresConcurrentIncrCardSeq(t *testing.T) {
	db := kanbantest.NewPostgresDB(t)
	ctx := context.Background()
	b := newBoard(t, db.Board(), "SEQ")
	store := db.Board()

	var wg sync.WaitGroup
	var failed atomic.Bool
	seqs := make(chan int64, incrSeqConcurrentWorkers)
	for range incrSeqConcurrentWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := store.IncrCardSeq(ctx, b.ID)
			if err != nil {
				failed.Store(true)
				return
			}
			seqs <- n
		}()
	}
	wg.Wait()
	close(seqs)
	if failed.Load() {
		t.Fatal("并发自增出现错误")
	}
	got := make([]int64, 0, incrSeqConcurrentWorkers)
	for n := range seqs {
		got = append(got, n)
	}
	slices.Sort(got)
	for i, n := range got {
		want := int64(i + 1)
		if n != want {
			t.Fatalf("编号集合不对，期望从 1 连续到 %d，实际 %v", incrSeqConcurrentWorkers, got)
		}
	}
}

// TestPostgresConcurrentClaimOneWinner 验证多 agent 同时领取同一张卡时只有一个成功。
func TestPostgresConcurrentClaimOneWinner(t *testing.T) {
	db := kanbantest.NewPostgresDB(t)
	core := kanbantest.NewCoreFromDB(t, db, kanban.WithClock(func() time.Time { return pgClaimNow }))
	ctx := context.Background()

	human := kanban.Actor{Kind: kanban.ActorHuman, Name: "admin"}
	detail, err := core.CreateBoard(ctx, human, kanban.CreateBoardInput{Name: "并发领取", Slug: "PGCLAIM", Locale: "zh-CN"})
	if err != nil {
		t.Fatalf("创建看板失败: %v", err)
	}
	if err := db.Rule().DeleteByBoard(ctx, detail.ID); err != nil {
		t.Fatalf("清规则失败: %v", err)
	}
	var doingID int64
	for _, col := range detail.Columns {
		if col.AgentPickup {
			doingID = col.ID
			break
		}
	}
	if doingID == 0 {
		t.Fatal("dev 模板里应该有 AgentPickup 列")
	}
	card, err := core.CreateCard(ctx, human, detail.ID, kanban.CreateCardInput{Title: "争用", ColumnID: doingID})
	if err != nil {
		t.Fatalf("创建卡片失败: %v", err)
	}

	var wg sync.WaitGroup
	var okCount atomic.Int32
	var claimedCount atomic.Int32
	for i := range claimConcurrentWorkers {
		wg.Add(1)
		agent := kanban.Actor{Kind: kanban.ActorAgent, Name: fmt.Sprintf("agent-%d", i)}
		go func(a kanban.Actor) {
			defer wg.Done()
			_, err := core.ClaimCard(ctx, a, card.ID, kanban.ClaimCardInput{LeaseMinutes: 30})
			if err == nil {
				okCount.Add(1)
				return
			}
			if errors.Is(err, kanban.ErrCardClaimed) {
				claimedCount.Add(1)
				return
			}
			t.Errorf("领取返回意外错误: %v", err)
		}(agent)
	}
	wg.Wait()

	if okCount.Load() != 1 {
		t.Fatalf("应该只有一个 agent 领取成功，实际成功 %d", okCount.Load())
	}
	got, err := db.Card().GetByID(ctx, card.ID)
	if err != nil || got.ClaimedBy == "" {
		t.Fatalf("卡片应该处于被领取状态: %+v %v", got, err)
	}
}

// TestPostgresUniqueErrorsMatchSQLite 在 PostgreSQL 上重复 slug 仍应被识别为 orm 唯一键冲突。
func TestPostgresUniqueErrorsMatchSQLite(t *testing.T) {
	kanbantest.ForEachBackend(t, func(t *testing.T, db kanbandb.DB) {
		store := db.Board()
		newBoard(t, store, "uniq-slug")
		err := store.Create(context.Background(), &kanban.Board{Name: "二", Slug: "uniq-slug"})
		if !orm.IsDuplicatedKey(err) {
			t.Fatalf("PostgreSQL 上也应把重复 slug 翻译成唯一键冲突，实际: %v", err)
		}
	})
}
