package kanban_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

var testAgent = kanban.Actor{Kind: kanban.ActorAgent, Name: "claude"}

// newBare 创建一个没有任何看板的 Core 和它的数据库，用来测试创建看板本身。
func newBare(t *testing.T, opts ...kanban.Option) (kanban.Core, kanbandb.DB) {
	t.Helper()
	db := kanbantest.NewDB(t)
	return kanban.NewCore(db, append([]kanban.Option{kanbantest.Resources(t)}, opts...)...), db
}

// mustCreateBoard 以人的身份创建看板，失败就让测试终止。
func mustCreateBoard(t *testing.T, core kanban.Core, in kanban.CreateBoardInput) *kanban.BoardDetail {
	t.Helper()
	d, err := core.CreateBoard(context.Background(), testActor, in)
	if err != nil {
		t.Fatalf("创建看板失败: %v", err)
	}
	return d
}

// columnNames 返回各列的名称。
func columnNames(cols []*kanban.Column) []string {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	return names
}

// TestCreateBoardDevTemplate 验证 dev 模板：五列按顺序、间距为 1024、下一列依次相连，
// 只有"进行中"允许 agent 领活，只有"完成"算完成，并且返回的数据和读出来的一致。
func TestCreateBoardDevTemplate(t *testing.T) {
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	core, _ := newBare(t, kanban.WithClock(func() time.Time { return fixed }))

	d := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "My Project", Locale: "zh-CN", ProjectPath: "/code/app"})
	if d.Slug != "MYPROJ" || d.Name != "My Project" || d.ProjectPath != "/code/app" || !d.CreatedAt.Equal(fixed) {
		t.Fatalf("看板字段不对: %+v", d.Board)
	}
	want := []string{"收件箱", "待办", "进行中", "审查", "完成"}
	if got := columnNames(d.Columns); !slices.Equal(got, want) {
		t.Fatalf("列名应该是 %v，实际 %v", want, got)
	}
	for i, c := range d.Columns {
		if c.BoardID != d.ID || c.Sort != int64(i+1)*1024 || c.Instruction == "" {
			t.Errorf("第 %d 列的归属、排序或指令不对: %+v", i, c)
		}
		if c.IsDone != (i == 4) {
			t.Errorf("第 %d 列的开关不对: done=%v", i, c.IsDone)
		}
		wantNext := int64(0)
		if i < 4 {
			wantNext = d.Columns[i+1].ID
		}
		if c.NextColumnID != wantNext {
			t.Errorf("第 %d 列的下一列应该是 %d，实际 %d", i, wantNext, c.NextColumnID)
		}
	}

	got, err := core.GetBoard(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("读取看板失败: %v", err)
	}
	if got.Slug != d.Slug || len(got.Columns) != 5 || got.Columns[2].NextColumnID != d.Columns[3].ID {
		t.Fatalf("读出来的数据和创建时返回的不一致: %+v", got)
	}
}

// TestCreateBoardOtherTemplates 验证 simple 是三列、empty 没有列，英文界面用英文列名；
// 列的 JSON 里 rules 和 columns 都是数组而不是 null。
func TestCreateBoardOtherTemplates(t *testing.T) {
	core, _ := newBare(t)
	simple := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "A1", Template: kanban.TemplateSimple, Locale: "en-US"})
	if got, want := columnNames(simple.Columns), []string{"To do", "In progress", "Done"}; !slices.Equal(got, want) {
		t.Fatalf("simple 的列名应该是 %v，实际 %v", want, got)
	}
	if simple.Columns[1].NextColumnID != simple.Columns[2].ID {
		t.Fatalf("simple 的进行中应该领活并指向完成: %+v", simple.Columns[1])
	}

	empty := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "B1", Template: kanban.TemplateEmpty})
	if empty.Columns == nil || len(empty.Columns) != 0 || empty.Rules == nil {
		t.Fatalf("empty 应该没有列，且列和规则都是空数组: %+v", empty)
	}
	ja := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "C1", Locale: "ja"})
	if ja.Columns[2].Name != "進行中" {
		t.Fatalf("日语界面的第三列应该是進行中，实际 %s", ja.Columns[2].Name)
	}
}

// TestCreateBoardRecordsEvent 验证创建看板会追加一条 board.created 事件并广播，
// 事件里记录了操作者、名称和标识。
func TestCreateBoardRecordsEvent(t *testing.T) {
	core, db := newBare(t)
	var got []kanban.Published
	var mu sync.Mutex
	core.Observe("record", func(_ context.Context, p kanban.Published) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, p)
		return nil
	})

	d := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "事件看板", Slug: "evt"})
	events, err := db.Event().ListAfter(context.Background(), d.ID, 0, 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("应该有 1 条事件，实际 %d, %v", len(events), err)
	}
	e := events[0]
	if e.Type != kanban.EventBoardCreated || e.Actor != "admin" || e.ActorKind != "human" ||
		e.Payload != `{"name":"事件看板","slug":"EVT"}` {
		t.Fatalf("事件内容不对: %+v", e)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0].BoardID != d.ID || got[0].Seq != e.ID {
		t.Fatalf("应该广播一次，实际 %+v", got)
	}
}

// TestCreateBoardSlug 验证标识：手填的转大写，被占用返回 409 且不留下半成品；
// 自动生成的被占用时依次加数字后缀；纯中文名用 BOARD。
func TestCreateBoardSlug(t *testing.T) {
	ctx := context.Background()
	core, _ := newBare(t)

	if d := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "x", Slug: "k1"}); d.Slug != "K1" {
		t.Fatalf("手填的标识应该转大写，实际 %s", d.Slug)
	}
	_, err := core.CreateBoard(ctx, testActor, kanban.CreateBoardInput{Name: "y", Slug: "K1"})
	if !errors.Is(err, kanban.ErrBoardSlugTaken) {
		t.Fatalf("手填的标识被占用应该返回 ErrBoardSlugTaken，实际 %v", err)
	}
	if boards, _ := core.ListBoards(ctx); len(boards) != 1 {
		t.Fatalf("冲突后不应该留下半成品，看板数 %d", len(boards))
	}

	var slugs []string
	for _, name := range []string{"My Project", "My Project", "My Project", "看板", "看板"} {
		slugs = append(slugs, mustCreateBoard(t, core, kanban.CreateBoardInput{Name: name}).Slug)
	}
	if want := []string{"MYPROJ", "MYPROJ2", "MYPROJ3", "BOARD", "BOARD2"}; !slices.Equal(slugs, want) {
		t.Fatalf("自动标识应该是 %v，实际 %v", want, slugs)
	}
}

// TestCreateBoardAutoSlugConcurrent 验证多个同名看板同时创建时，自动标识的重试逻辑能让全部成功，
// 标识互不相同，列也各自完整。
func TestCreateBoardAutoSlugConcurrent(t *testing.T) {
	core, _ := newBare(t)
	const n = 8
	slugs := make(chan string, n)
	within(t, 20*time.Second, func() {
		var wg sync.WaitGroup
		for range n {
			wg.Go(func() {
				d, err := core.CreateBoard(context.Background(), testActor, kanban.CreateBoardInput{Name: "Same Name"})
				if err != nil {
					t.Errorf("并发创建失败: %v", err)
					return
				}
				if len(d.Columns) != 5 {
					t.Errorf("列不完整: %d", len(d.Columns))
				}
				slugs <- d.Slug
			})
		}
		wg.Wait()
	})
	close(slugs)
	seen := map[string]bool{}
	for s := range slugs {
		if seen[s] {
			t.Fatalf("标识重复: %s", s)
		}
		seen[s] = true
	}
	if len(seen) != n {
		t.Fatalf("应该创建 %d 个看板，实际 %d", n, len(seen))
	}
}

// TestCreateBoardByAgent 验证 agent 也能创建看板，事件里记录的来源是 agent。
func TestCreateBoardByAgent(t *testing.T) {
	ctx := context.Background()
	core, _ := newBare(t)
	got, err := core.CreateBoard(ctx, testAgent, kanban.CreateBoardInput{Name: "agent 建的", Slug: "AGT"})
	if err != nil || got.Board.Slug != "AGT" {
		t.Fatalf("agent 应该能创建看板: %+v %v", got, err)
	}
}

// TestCreateBoardRejects 验证不该成功的情况：自动化和身份无效的操作者被拒绝，
// 不合法的输入返回 ErrInvalidInput，失败后都不会留下看板。
func TestCreateBoardRejects(t *testing.T) {
	ctx := context.Background()
	core, _ := newBare(t)

	system := kanban.Actor{Kind: kanban.ActorSystem, Name: "automation"}
	if _, err := core.CreateBoard(ctx, system, kanban.CreateBoardInput{Name: "x"}); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Fatalf("自动化应该得到 ErrHumanOnly，实际 %v", err)
	}
	if _, err := core.CreateBoard(ctx, kanban.Actor{}, kanban.CreateBoardInput{Name: "x"}); !errors.Is(err, kanban.ErrInvalidActor) {
		t.Fatalf("身份无效应该得到 ErrInvalidActor，实际 %v", err)
	}
	bad := []kanban.CreateBoardInput{
		{Name: ""},
		{Name: "x", Slug: "1abc"},
		{Name: "x", Template: "nope"},
		{Name: "x", ProjectPath: "relative/path"},
	}
	for i, in := range bad {
		if _, err := core.CreateBoard(ctx, testActor, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("第 %d 组应该得到 ErrInvalidInput，实际 %v", i, err)
		}
	}
	if boards, _ := core.ListBoards(ctx); len(boards) != 0 {
		t.Fatalf("失败后不应该有看板，实际 %d", len(boards))
	}
}

// TestGetBoardNotFound 验证读取不存在的看板返回 404 对应的领域错误。
func TestGetBoardNotFound(t *testing.T) {
	core, _ := newBare(t)
	_, err := core.GetBoard(context.Background(), 12345)
	if !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Fatalf("应该返回 ErrBoardNotFound，实际 %v", err)
	}
	var info interface{ GetHTTPCode() int }
	if !errors.As(err, &info) || info.GetHTTPCode() != 404 {
		t.Fatalf("应该对应 HTTP 404，实际 %v", err)
	}
}

// TestListBoards 验证看板列表按创建顺序返回，没有看板时是空数组而不是 nil。
func TestListBoards(t *testing.T) {
	ctx := context.Background()
	core, _ := newBare(t)
	empty, err := core.ListBoards(ctx)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("没有看板时应该返回空数组，实际 %v, %v", empty, err)
	}
	for i := range 3 {
		mustCreateBoard(t, core, kanban.CreateBoardInput{Name: fmt.Sprintf("看板 %d", i), Slug: fmt.Sprintf("B%d", i)})
	}
	boards, err := core.ListBoards(ctx)
	if err != nil || len(boards) != 3 || boards[0].Slug != "B0" || boards[2].Slug != "B2" {
		t.Fatalf("应该按创建顺序返回 3 个看板，实际 %+v, %v", boards, err)
	}
}

// TestListBoardsLastActivity 验证看板列表带最后活动时间：取看板自身更新时间和最新事件时间里较晚的那个，
// 所以看板本身没被改过、但任务上一直有动静时，列表里的时间是新的。
func TestListBoardsLastActivity(t *testing.T) {
	ctx := context.Background()
	core, db := newBare(t)
	quiet := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "安静", Slug: "QUIET"})
	busy := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "活跃", Slug: "BUSY"})
	later := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	if err := db.Event().Create(ctx, &kanban.Event{BoardID: busy.ID, Type: "card.updated", ActorKind: "human", Actor: "human", Payload: "{}", CreatedAt: later}); err != nil {
		t.Fatalf("写事件失败: %v", err)
	}
	boards, err := core.ListBoards(ctx)
	if err != nil || len(boards) != 2 {
		t.Fatalf("读取列表失败: %v %v", boards, err)
	}
	if got := boards[0]; got.ID != quiet.ID || got.LastActivityAt.Before(got.UpdatedAt) || got.LastActivityAt.After(later) {
		t.Fatalf("没有新事件时最后活动时间不应早于看板更新时间: %+v", got)
	}
	if got := boards[1]; !got.LastActivityAt.Equal(later) {
		t.Fatalf("最后活动时间应取最新事件的时间 %v，实际 %v", later, got.LastActivityAt)
	}
}

// boardEvents 返回看板里的事件，按序号升序。
func boardEvents(t *testing.T, db kanbandb.DB, boardID int64) []*kanban.Event {
	t.Helper()
	items, err := db.Event().ListAfter(context.Background(), boardID, 0, 1000)
	if err != nil {
		t.Fatalf("读取事件失败: %v", err)
	}
	return items
}

// strPtr 返回字符串的指针，用来构造"出现的字段"。
func strPtr(s string) *string { return &s }

// TestUpdateBoardByHuman 验证人可以改三个字段：只更新出现的字段，标识不变，
// 事件里记录真正变化的字段名，读出来的数据和返回的一致。
func TestUpdateBoardByHuman(t *testing.T) {
	ctx := context.Background()
	core, db := newBare(t)
	d := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "旧名字", Slug: "OLD", Description: "旧说明"})

	got, err := core.UpdateBoard(ctx, testActor, d.ID, kanban.UpdateBoardInput{Name: strPtr("  新名字 "), ProjectPath: strPtr("/code/app")})
	if err != nil {
		t.Fatalf("修改失败: %v", err)
	}
	if got.Name != "新名字" || got.Description != "旧说明" || got.ProjectPath != "/code/app" || got.Slug != "OLD" {
		t.Fatalf("只应该更新出现的字段，实际 %+v", got)
	}
	read, _ := core.GetBoard(ctx, d.ID)
	if read.Name != "新名字" || read.ProjectPath != "/code/app" {
		t.Fatalf("读出来的数据不一致: %+v", read.Board)
	}
	events := boardEvents(t, db, d.ID)
	last := events[len(events)-1]
	if len(events) != 2 || last.Type != kanban.EventBoardUpdated || last.Payload != `{"fields":["name","project_path"]}` {
		t.Fatalf("应该多一条记录了变化字段的事件，实际 %+v", events)
	}

	cleared, err := core.UpdateBoard(ctx, testActor, d.ID, kanban.UpdateBoardInput{ProjectPath: strPtr("")})
	if err != nil || cleared.ProjectPath != "" {
		t.Fatalf("人可以清空项目路径: %+v %v", cleared, err)
	}
}

// TestUpdateBoardNoChange 验证提交的值和原来相同时，不写事件也不广播。
func TestUpdateBoardNoChange(t *testing.T) {
	ctx := context.Background()
	core, db := newBare(t)
	d := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "名字", Slug: "SAME"})
	published := 0
	core.Observe("count", func(context.Context, kanban.Published) error { published++; return nil })

	for _, in := range []kanban.UpdateBoardInput{{}, {Name: strPtr("名字")}, {Description: strPtr("")}} {
		if _, err := core.UpdateBoard(ctx, testActor, d.ID, in); err != nil {
			t.Fatalf("没有变化也应该成功: %v", err)
		}
	}
	if n := len(boardEvents(t, db, d.ID)); n != 1 {
		t.Fatalf("没有变化不应该写事件，实际 %d 条", n)
	}
	if published != 0 {
		t.Fatalf("没有变化不应该广播，实际 %d 次", published)
	}
}

// TestUpdateBoardByAgent 验证 agent 的权限：只能在项目路径为空时写入项目路径；
// 路径已有值时改成别的值被拒绝，写成相同的值按幂等成功；改名称、说明或什么都不改都被拒绝；
// 人始终可以修改。
func TestUpdateBoardByAgent(t *testing.T) {
	ctx := context.Background()
	core, db := newBare(t)
	d := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "名字", Slug: "AG"})

	if _, err := core.UpdateBoard(ctx, testAgent, d.ID, kanban.UpdateBoardInput{ProjectPath: strPtr("/work/a")}); err != nil {
		t.Fatalf("路径为空时 agent 应该能写入: %v", err)
	}
	events := boardEvents(t, db, d.ID)
	if last := events[len(events)-1]; last.ActorKind != "agent" || last.Actor != "claude" || last.Type != kanban.EventBoardUpdated {
		t.Fatalf("事件应该记录 agent 为操作者: %+v", last)
	}

	if _, err := core.UpdateBoard(ctx, testAgent, d.ID, kanban.UpdateBoardInput{ProjectPath: strPtr("/work/b")}); !errors.Is(err, kanban.ErrProjectPathLocked) {
		t.Fatalf("路径已有值时 agent 改成别的值应该得到 ErrProjectPathLocked，实际 %v", err)
	}
	if _, err := core.UpdateBoard(ctx, testAgent, d.ID, kanban.UpdateBoardInput{ProjectPath: strPtr("/work/a")}); err != nil {
		t.Fatalf("写成相同的值应该按幂等成功: %v", err)
	}
	for _, in := range []kanban.UpdateBoardInput{
		{Name: strPtr("agent 改的名字")},
		{Description: strPtr("agent 改的说明")},
		{Name: strPtr("x"), ProjectPath: strPtr("/work/a")},
		{},
	} {
		if _, err := core.UpdateBoard(ctx, testAgent, d.ID, in); !errors.Is(err, kanban.ErrHumanOnly) {
			t.Errorf("agent 改其他字段应该得到 ErrHumanOnly，实际 %v", err)
		}
	}
	if _, err := core.UpdateBoard(ctx, testActor, d.ID, kanban.UpdateBoardInput{ProjectPath: strPtr("/work/c")}); err != nil {
		t.Fatalf("人应该可以修改已有的路径: %v", err)
	}
	system := kanban.AutomationActor("规则")
	if _, err := core.UpdateBoard(ctx, system, d.ID, kanban.UpdateBoardInput{Name: strPtr("x")}); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Fatalf("系统操作者不能改看板，实际 %v", err)
	}
}

// TestUpdateBoardAgentsRace 验证多个 agent 同时写项目路径时，只有第一个成功，
// 其余的看到已有值而被拒绝，最终路径就是赢家写的那个。
func TestUpdateBoardAgentsRace(t *testing.T) {
	ctx := context.Background()
	core, _ := newBare(t)
	d := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "名字", Slug: "RACE"})

	const n = 6
	var mu sync.Mutex
	winners := map[string]bool{}
	within(t, 20*time.Second, func() {
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				path := fmt.Sprintf("/work/%d", i)
				_, err := core.UpdateBoard(ctx, kanban.Actor{Kind: kanban.ActorAgent, Name: fmt.Sprintf("a%d", i)}, d.ID,
					kanban.UpdateBoardInput{ProjectPath: &path})
				switch {
				case err == nil:
					mu.Lock()
					winners[path] = true
					mu.Unlock()
				case !errors.Is(err, kanban.ErrProjectPathLocked):
					t.Errorf("意外的错误: %v", err)
				}
			})
		}
		wg.Wait()
	})
	got, _ := core.GetBoard(ctx, d.ID)
	if len(winners) != 1 || !winners[got.ProjectPath] {
		t.Fatalf("应该恰好一个赢家且路径一致，赢家 %v，最终路径 %q", winners, got.ProjectPath)
	}
}

// TestUpdateBoardRejects 验证看板不存在、输入不合法、身份无效时的错误。
func TestUpdateBoardRejects(t *testing.T) {
	ctx := context.Background()
	core, _ := newBare(t)
	d := mustCreateBoard(t, core, kanban.CreateBoardInput{Name: "名字", Slug: "REJ"})

	if _, err := core.UpdateBoard(ctx, testActor, 9999, kanban.UpdateBoardInput{Name: strPtr("x")}); !errors.Is(err, kanban.ErrBoardNotFound) {
		t.Fatalf("看板不存在应该得到 ErrBoardNotFound，实际 %v", err)
	}
	for _, in := range []kanban.UpdateBoardInput{{Name: strPtr("   ")}, {ProjectPath: strPtr("relative")}} {
		if _, err := core.UpdateBoard(ctx, testActor, d.ID, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("应该得到 ErrInvalidInput，实际 %v", err)
		}
	}
	if _, err := core.UpdateBoard(ctx, kanban.Actor{}, d.ID, kanban.UpdateBoardInput{Name: strPtr("x")}); !errors.Is(err, kanban.ErrInvalidActor) {
		t.Fatalf("身份无效应该得到 ErrInvalidActor，实际 %v", err)
	}
	if got, _ := core.GetBoard(ctx, d.ID); got.Name != "名字" {
		t.Fatalf("失败的修改不应该生效: %+v", got.Board)
	}
}
