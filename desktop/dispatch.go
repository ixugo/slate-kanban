package desktop

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"slate-kanban/internal/core/kanban"
)

const (
	dispatchQueueSize    = 128
	dispatchLeaseMinutes = 30
)

type dispatchConfig struct {
	BoardID int64            `json:"board_id"`
	Hosts   map[int64]string `json:"hosts"`
	Enabled bool             `json:"enabled"`
}

type dispatchFlow struct {
	inbox, todo, dev, review, done int64
}

// A development template has five columns connected in workflow order.
// Require the template stage names as well, so a custom board is not dispatched
// just because it happens to contain five columns.
func templateFlow(columns []*kanban.Column) (dispatchFlow, error) {
	names := [][]string{
		{"收件箱", "收件匣", "Inbox", "受信トレイ", "받은 편지함"},
		{"待办", "待辦", "To do", "未着手", "할 일"},
		{"进行中", "進行中", "In progress", "진행 중"},
		{"审查", "審查", "Review", "レビュー", "검토"},
		{"完成", "Done", "完了", "완료"},
	}
	if len(columns) != len(names) {
		return dispatchFlow{}, errDispatchTemplate
	}
	for i, col := range columns {
		last := i == len(columns)-1
		if !slices.Contains(names[i], col.Name) || col.IsDone != last || (!last && col.NextColumnID != columns[i+1].ID) {
			return dispatchFlow{}, errDispatchTemplate
		}
	}
	return dispatchFlow{columns[0].ID, columns[1].ID, columns[2].ID, columns[3].ID, columns[4].ID}, nil
}

var errDispatchTemplate = errors.New("desktop dispatch requires a development board template")

type dispatchCore interface {
	BoardSnapshot(context.Context, int64) (*kanban.Snapshot, error)
	ExportBoard(context.Context, kanban.Actor, int64) (*kanban.BoardExport, error)
	ListEvents(context.Context, int64, int64, int) (*kanban.EventPage, error)
	Subscribe(int64) (<-chan struct{}, func())
	GetCardDetail(context.Context, int64) (*kanban.CardDetail, error)
	ClaimCard(context.Context, kanban.Actor, int64, kanban.ClaimCardInput) (*kanban.CardDetail, error)
	MoveCard(context.Context, kanban.Actor, int64, kanban.MoveCardInput) (*kanban.Card, error)
	UpdateCard(context.Context, kanban.Actor, int64, kanban.UpdateCardInput) (*kanban.Card, error)
	AddComment(context.Context, kanban.Actor, int64, string) (*kanban.Comment, error)
	AddCommentWithExt(context.Context, kanban.Actor, int64, string, string) (*kanban.Comment, error)
	FinishCardWithExt(context.Context, kanban.Actor, int64, kanban.FinishCardInput, string) (*kanban.CardDetail, error)
	ReleaseCard(context.Context, kanban.Actor, int64) error
}

type engineerStatus struct {
	State  string    `json:"state"`
	CardID int64     `json:"card_id,omitzero"`
	Retry  time.Time `json:"retry_at,omitzero"`
}

type dispatchJob struct {
	id       int64
	review   bool
	engineer kanban.Engineer
	host     string
	flow     dispatchFlow
	project  string
}

type dispatchCompletion struct {
	job     dispatchJob
	block   bool
	recheck bool
}

type dispatchRun struct {
	core     dispatchCore
	config   dispatchConfig
	host     dispatchHost
	text     func() desktopText
	sessions string
	update   func(int64, engineerStatus)
	// renewEvery 是后台检查任务归属、续租约的间隔，零值用 dispatchRenewEvery；测试里调小以免干等。
	renewEvery time.Duration

	dev, review chan dispatchJob
	completed   chan dispatchCompletion
}

func (d *dispatchRun) run(ctx context.Context, snapshot *kanban.Snapshot) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	d.dev = make(chan dispatchJob, dispatchQueueSize)
	d.review = make(chan dispatchJob, dispatchQueueSize)
	d.completed = make(chan dispatchCompletion, 2*dispatchQueueSize)
	var workers sync.WaitGroup
	defer workers.Wait()
	defer cancel()

	for _, engineer := range snapshot.Engineers {
		if d.config.Hosts[engineer.ID] == "" {
			continue
		}
		queue := d.dev
		if engineer.Kind == kanban.EngineerReview {
			queue = d.review
		}
		workers.Go(func() { d.worker(ctx, queue) })
	}

	changed, unsubscribe := d.core.Subscribe(d.config.BoardID)
	defer unsubscribe()
	// Subscribe before refreshing the initial snapshot to cover the gap.
	snapshot, err := d.core.BoardSnapshot(ctx, d.config.BoardID)
	if err != nil {
		return
	}
	seq := snapshot.Seq
	active := make(map[int64]int64)
	blockedVersion := make(map[int64]int64)
	for {
		flow, err := templateFlow(snapshot.Columns)
		if err != nil {
			return
		}
		d.schedule(snapshot, flow, active, blockedVersion)
		needSnapshot := false
		select {
		case <-ctx.Done():
			return
		case result := <-d.completed:
			delete(active, result.job.engineer.ID)
			if result.recheck {
				// A change may already have been consumed while the card
				// was active. Let the fresh snapshot schedule it again.
				delete(blockedVersion, result.job.id)
			} else if result.block || result.job.review {
				d.versionAfterRun(ctx, result.job, blockedVersion)
			}
			needSnapshot = true
		case <-changed:
			// 快照里带着 project_path，看板字段一改就要重拉，不能只等任务事件。
			needSnapshot = true
		case <-time.After(10 * time.Second):
			// Hub notifications are process-local. Check the event tail
			// when another desktop writes to a shared database.
			needSnapshot = true
		}
		for {
			page, err := d.core.ListEvents(ctx, d.config.BoardID, seq, 200)
			if err != nil {
				break
			}
			needSnapshot = needSnapshot || page.Truncated
			for _, ev := range page.Events {
				seq = ev.ID
				if ev.Type != kanban.EventCommentAdded || ev.ActorKind == string(kanban.ActorHuman) {
					needSnapshot = true
				}
				if ev.ActorKind == string(kanban.ActorHuman) {
					delete(blockedVersion, ev.CardID)
				}
			}
			if !page.HasMore {
				seq = max(seq, page.Latest)
				break
			}
		}
		if needSnapshot {
			if current, err := d.core.BoardSnapshot(ctx, d.config.BoardID); err == nil {
				snapshot = current
			}
		}
	}
}

// versionAfterRun 在一次派发结束后记下任务当前版本，调度器在版本变化前不再重复派发它，
// 避免失败或退回的任务被不停地重新领取。研发交接到审查列的任务例外，要让审查马上看到。
func (d *dispatchRun) versionAfterRun(ctx context.Context, job dispatchJob, blocked map[int64]int64) {
	card, err := d.core.GetCardDetail(ctx, job.id)
	// 合并冲突退回研发的任务要让研发立刻接手，不能按"审查退回"等人来动它。
	conflictReturned := err == nil && job.review && card.ColumnID == job.flow.dev && rebaseTarget(card) != ""
	if err == nil && !conflictReturned && (card.ColumnID == job.flow.dev || card.ColumnID == job.flow.todo || card.ColumnID == job.flow.review) {
		// Development handoff must be visible to a review worker immediately.
		if job.review || card.ColumnID != job.flow.review {
			blocked[job.id] = card.Version
		}
	}
}

func (d *dispatchRun) schedule(snapshot *kanban.Snapshot, flow dispatchFlow, active, blockedVersion map[int64]int64) {
	busy := make(map[int64]bool)
	for _, card := range snapshot.Cards {
		if card.ClaimActive {
			busy[card.ClaimEngineerID] = true
		}
	}
	for _, column := range []int64{flow.dev, flow.todo, flow.review} {
		isReview := column == flow.review
		for _, card := range snapshot.Cards {
			version, blocked := blockedVersion[card.ID]
			if card.ColumnID != column || card.Waiting || card.HumanOnly || card.ClaimActive ||
				(blocked && version == card.Version) || slices.ContainsFunc(snapshot.Engineers, func(e *kanban.Engineer) bool { return active[e.ID] == card.ID }) {
				continue
			}
			for _, e := range snapshot.Engineers {
				host := d.config.Hosts[e.ID]
				if host == "" || (e.Kind == kanban.EngineerReview) != isReview || active[e.ID] != 0 || busy[e.ID] ||
					(!isReview && card.EngineerID != 0 && card.EngineerID != e.ID) {
					continue
				}
				queue := d.dev
				if isReview {
					queue = d.review
				}
				select {
				case queue <- dispatchJob{card.ID, isReview, *e, host, flow, snapshot.ProjectPath}:
					active[e.ID] = card.ID
				default:
				}
				break
			}
		}
	}
}

func (d *dispatchRun) worker(ctx context.Context, queue <-chan dispatchJob) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-queue:
			if ctx.Err() != nil {
				return
			}
			d.update(job.engineer.ID, engineerStatus{State: "working", CardID: job.id})
			err := d.handle(ctx, job)
			recheck := errors.Is(err, errDispatchPlanSuperseded)
			block := err != nil && !errors.Is(err, context.Canceled) && !recheck
			status := engineerStatus{State: "idle"}
			if block {
				status = engineerStatus{State: "blocked", CardID: job.id}
			}
			d.update(job.engineer.ID, status)
			select {
			case d.completed <- dispatchCompletion{job: job, block: block, recheck: recheck}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// actor 返回这位工程师在看板上留下记录时使用的身份，名字就是工程师名。
// 每个 worker 只处理自己的工程师，所以领取、移列和评论都直接记在这位工程师名下。
func (j dispatchJob) actor() kanban.Actor {
	return kanban.Actor{Kind: kanban.ActorAgent, Name: j.engineer.Name}
}
