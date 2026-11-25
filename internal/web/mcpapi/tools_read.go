package mcpapi

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

// boardIn 是 get_board 的入参。
type boardIn struct {
	BoardID            int64 `json:"board_id" jsonschema:"看板编号"`
	IncludeWatchPrompt *bool `json:"include_watch_prompt,omitempty" jsonschema:"缺省返回守候说明；传 false 省略"`
}

type listBoardsIn struct {
	Summary bool `json:"summary,omitempty" jsonschema:"仅返回看板编号、名称、标识、项目路径"`
}

// cardIn 是只需要任务编号的工具的入参。
type cardIn struct {
	CardID int64 `json:"card_id" jsonschema:"任务编号，是数字 id，不是 SLATE-12 这样的 key；list_cards 的结果里两个都有"`
}

// listCardsIn 是 list_cards 的入参。
type listCardsIn struct {
	BoardID int64  `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	Query   string `json:"query,omitempty" jsonschema:"搜索词和筛选条件。关键词匹配标题、描述、编号或标签。不填列出全部"`
	Limit   int    `json:"limit,omitempty" jsonschema:"最多返回几个，默认 10，最多 30；还有更多时 truncated 为 true，收紧筛选条件再取下一批"`
}

// whoamiOut 是 whoami 的输出。
type whoamiOut struct {
	Name     string           `json:"name"`
	Kind     kanban.ActorKind `json:"kind"`
	View     view             `json:"view"`
	Version  string           `json:"server_version"`
	BoardURI string           `json:"board_resource_template"`
}

// boardsOut 是 list_boards 的输出，列表外面包一层对象，因为结构化输出要求是对象。
type boardsOut struct {
	Boards []*kanban.Board `json:"boards"`
}

type boardSummary struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	ProjectPath string `json:"project_path"`
}

type boardSummariesOut struct {
	Boards []boardSummary `json:"boards"`
}

// cardsOut 是 list_cards 的输出：摘要列表，加上命中总数和是否被截断，调用方可以收紧筛选条件再查。
type cardsOut struct {
	Cards     []cardSummary `json:"cards"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated"`
}

// registerReadTools 注册所有连接共用的读取工具。
func (a API) registerReadTools(s *mcp.Server, v view) {
	register(s, spec{
		name: "whoami", kind: kindRead,
		desc: "核对身份、工具视图、服务版本与看板资源 URI 模板。",
	}, a.whoami(v))
	register(s, spec{
		name: "list_boards", kind: kindRead,
		desc: "列出看板。只需定位项目时传 summary=true；详细设置用 get_board。",
	}, a.listBoards)
	register(s, spec{
		name: "get_board", kind: kindRead,
		desc: "读取看板、列指令、自动化与工程师名册。守候用 watch_prompt，普通查询可省略。",
	}, a.getBoard)
	register(s, spec{
		name: "list_cards", kind: kindRead,
		desc: "搜索或筛选看板任务，按看板顺序返回摘要。query 支持引号短语、减号取反、字段过滤和竖线多选。" +
			"例如 col:进行中 is:free -is:waiting。完整语法见 skill。",
	}, a.listCards)
	register(s, spec{
		name: "get_card", kind: kindRead,
		desc: "读取任务全文、评论、依赖和列指令。按最新评论及列指令处理。",
	}, a.getCard)

}

// whoami 返回调用者的身份和当前视图。身份来自 URL 的 ?name= / ?role=，不来自入参，所以没有入参。
func (a API) whoami(v view) func(context.Context, kanban.Actor, struct{}) (any, error) {
	return func(_ context.Context, actor kanban.Actor, _ struct{}) (any, error) {
		return whoamiOut{
			Name: actor.Name, Kind: actor.Kind, View: v,
			Version: a.version, BoardURI: boardURITemplate,
		}, nil
	}
}

// listBoards 返回全部看板。没有看板时是空数组，不是 null。
func (a API) listBoards(ctx context.Context, _ kanban.Actor, in listBoardsIn) (any, error) {
	boards, err := a.core.ListBoards(ctx)
	if err != nil {
		return nil, err
	}
	if in.Summary {
		out := make([]boardSummary, 0, len(boards))
		for _, board := range boards {
			out = append(out, boardSummary{ID: board.ID, Name: board.Name, Slug: board.Slug, ProjectPath: board.ProjectPath})
		}
		return boardSummariesOut{Boards: out}, nil
	}
	if boards == nil {
		boards = []*kanban.Board{}
	}
	return boardsOut{Boards: boards}, nil
}

// getBoard 返回看板、列和规则，列里带着给 agent 的指令，并附上这个看板的守候提示词 watch_prompt。
func (a API) getBoard(ctx context.Context, _ kanban.Actor, in boardIn) (any, error) {
	detail, err := a.core.GetBoard(ctx, in.BoardID)
	if err != nil {
		return nil, err
	}
	out := boardOut{BoardDetail: detail}
	if in.IncludeWatchPrompt == nil || *in.IncludeWatchPrompt {
		if skillsAware(ctx) {
			out.WatchPrompt = skillWatchPromptOf(detail)
		} else {
			out.WatchPrompt = watchPromptOf(detail)
		}
	}
	return out, nil
}

// listCards 按筛选条件列出任务摘要。数量默认 10、最多 30，保护 agent 的上下文。
func (a API) listCards(ctx context.Context, _ kanban.Actor, in listCardsIn) (any, error) {
	list, err := a.core.ListCards(ctx, in.BoardID, kanban.ListCardsInput{Query: in.Query, Limit: clampListLimit(in.Limit)})
	if err != nil {
		return nil, err
	}
	return cardsOut{Cards: summarizeAll(list.Cards), Total: list.Total, Truncated: list.Truncated}, nil
}

// getCard 返回任务的完整信息：全文、评论、依赖和所在列的指令，并附上「任务不清楚先反馈」的提示。
func (a API) getCard(ctx context.Context, _ kanban.Actor, in cardIn) (any, error) {
	detail, err := a.core.GetCardDetail(ctx, in.CardID)
	if err != nil {
		return nil, err
	}
	return withNotice(detail), nil
}
