package mcpapi

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

// boardIn 是只需要看板编号的工具的入参。
type boardIn struct {
	BoardID int64 `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
}

// cardIn 是只需要卡片编号的工具的入参。
type cardIn struct {
	CardID int64 `json:"card_id" jsonschema:"卡片编号，是数字 id，不是 SLATE-12 这样的 key；list_cards 的结果里两个都有"`
}

// listCardsIn 是 list_cards 的入参。
type listCardsIn struct {
	BoardID int64  `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	Query   string `json:"query,omitempty" jsonschema:"筛选条件，写法见工具说明；不填列出全部"`
	Limit   int    `json:"limit,omitempty" jsonschema:"最多返回几张，默认 50，最多 200"`
}

// listEventsIn 是 list_events 的入参。
type listEventsIn struct {
	BoardID int64 `json:"board_id" jsonschema:"看板编号，来自 list_boards"`
	After   int64 `json:"after,omitempty" jsonschema:"起点事件序号，只返回比它大的事件；不填从最早保留的事件开始"`
	Limit   int   `json:"limit,omitempty" jsonschema:"最多返回几条，不填用默认值"`
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

// cardsOut 是 list_cards 的输出：摘要列表，加上命中总数和是否被截断，调用方可以收紧筛选条件再查。
type cardsOut struct {
	Cards     []cardSummary `json:"cards"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated"`
}

// registerReadTools 注册读取类的工具：两个视图共用的几个，加上只有管理视图才有的活动流水。
func (a API) registerReadTools(s *mcp.Server, v view) {
	register(s, spec{
		name: "whoami", kind: kindRead,
		desc: "返回你当前的身份（名称和类型）、使用的工具视图、服务版本，以及订阅看板更新用的资源地址模板。连上之后先调用它确认自己是谁。",
	}, a.whoami(v))
	register(s, spec{
		name: "list_boards", kind: kindRead,
		desc: "列出所有看板（编号、标识、名称、说明、项目路径）。其他工具都要用看板编号 board_id，先用它找到要处理的看板。",
	}, a.listBoards)
	register(s, spec{
		name: "get_board", kind: kindRead,
		desc: "读取一个看板的详情：全部列和自动化规则。每一列有 description（给人看的说明）、" +
			"instruction（人写给 agent 的做事规则，必须遵守）、agent_pickup（为 true 表示 agent 只在这类列里领取卡片）、" +
			"is_done（完成列）、next_column_id（交接后卡片去的下一列）。开始工作前先读一遍。",
	}, a.getBoard)
	register(s, spec{
		name: "list_cards", kind: kindRead,
		desc: "列出看板里的卡片摘要，顺序和看板上看到的一致（先按列从左到右，同一列里从上到下）。摘要不含描述和评论，要全文请用 get_card。" +
			"query 是空格分隔的多个词，所有词都满足才命中，词前加减号表示取反，双引号包起来的算一个词。写法：" +
			"普通文字（匹配标题、描述、编号、标签）；tag:标签；col:列名（也可以写列 id）；key:SLATE-12；title:文字；desc:文字；" +
			"priority:high、priority:3、priority:>=2（high、medium、low、none 对应 3、2、1、0）；claimed:agent 名称；" +
			"is:done、is:open、is:human、is:claimed、is:free、is:waiting、is:overdue、is:started、is:finished；" +
			"has:due、has:branch、has:url、has:tags、has:desc、has:deps（no: 表示没有）；due:today、due:<2026-12-01（created:、updated: 同理）。" +
			"例如：col:进行中 is:free -is:waiting。",
	}, a.listCards)
	register(s, spec{
		name: "get_card", kind: kindRead,
		desc: "读取一张卡片的完整信息：全文描述、评论、依赖的卡片，以及所在列给 agent 的指令（column_instruction）。" +
			"领到卡片后、动手之前先读它，评论里可能有人的补充要求或审查意见。",
	}, a.getCard)
	if v == viewAdmin {
		register(s, spec{
			name: "list_events", kind: kindRead,
			desc: "读取看板的活动流水：起点 after 之后的事件，按序号升序，不做过滤。用来了解看板最近发生了什么。" +
				"latest 是看板当前最新的事件序号；has_more 为真表示还有更多，用最后一条的 id 作为下一次的 after 继续读。",
		}, a.listEvents)
	}
}

// whoami 返回调用者的身份和当前视图。身份来自令牌，不来自入参，所以没有入参。
func (a API) whoami(v view) func(context.Context, kanban.Actor, struct{}) (any, error) {
	return func(_ context.Context, actor kanban.Actor, _ struct{}) (any, error) {
		return whoamiOut{
			Name: actor.Name, Kind: actor.Kind, View: v,
			Version: a.version, BoardURI: boardURITemplate,
		}, nil
	}
}

// listBoards 返回全部看板。没有看板时是空数组，不是 null。
func (a API) listBoards(ctx context.Context, _ kanban.Actor, _ struct{}) (any, error) {
	boards, err := a.core.ListBoards(ctx)
	if err != nil {
		return nil, err
	}
	if boards == nil {
		boards = []*kanban.Board{}
	}
	return boardsOut{Boards: boards}, nil
}

// getBoard 返回看板、列和规则，列里带着给 agent 的指令。
func (a API) getBoard(ctx context.Context, _ kanban.Actor, in boardIn) (any, error) {
	return a.core.GetBoard(ctx, in.BoardID)
}

// listCards 按筛选条件列出卡片摘要。数量默认 50、最多 200，保护 agent 的上下文。
func (a API) listCards(ctx context.Context, _ kanban.Actor, in listCardsIn) (any, error) {
	list, err := a.core.ListCards(ctx, in.BoardID, kanban.ListCardsInput{Query: in.Query, Limit: clampListLimit(in.Limit)})
	if err != nil {
		return nil, err
	}
	return cardsOut{Cards: summarizeAll(list.Cards), Total: list.Total, Truncated: list.Truncated}, nil
}

// getCard 返回卡片的完整信息：全文、评论、依赖和所在列的指令。
func (a API) getCard(ctx context.Context, _ kanban.Actor, in cardIn) (any, error) {
	return a.core.GetCardDetail(ctx, in.CardID)
}

// listEvents 返回起点之后的活动流水，不做过滤，只有管理视图提供。
func (a API) listEvents(ctx context.Context, _ kanban.Actor, in listEventsIn) (any, error) {
	return a.core.ListEvents(ctx, in.BoardID, in.After, in.Limit)
}
