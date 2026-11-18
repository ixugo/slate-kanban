// 和后端 JSON 一一对应的类型，来源是 internal/core/kanban 下的 *.model.go、*.param.go，
// 以及 internal/web/api/connectapi。后端改字段时，这里要一起改。
// 时间一律是 RFC3339 字符串；可空的时间是 null。

/** 操作者类型：人、agent、系统（自动化规则） */
export type ActorKind = 'human' | 'agent' | 'system'

export interface Board {
  id: number
  name: string
  /** 卡片编号的前缀，创建后不能改 */
  slug: string
  description: string
  /** 看板关联的本地项目路径，agent 建 worktree 时用 */
  project_path: string
  created_at: string
  updated_at: string
}

export interface Column {
  id: number
  board_id: number
  name: string
  /** 给人看的说明 */
  description: string
  /** 给 agent 的指令：这一列的卡片该怎么处理 */
  instruction: string
  /** 是否允许 agent 领取这一列的卡片 */
  agent_pickup: boolean
  /** 卡片进入这一列算完成 */
  is_done: boolean
  /** 完成后默认去哪一列，0 表示没有 */
  next_column_id: number
  sort: number
  created_at: string
  updated_at: string
}

/** 规则的触发类型 */
export type TriggerType = 'card_created' | 'card_entered' | 'card_left'

/** 规则的动作类型，和服务端 rule.go 里的 Action* 常量一致 */
export type ActionType =
  | 'set_started_at'
  | 'set_finished_at'
  | 'clear_started_at'
  | 'clear_finished_at'
  | 'set_due'
  | 'clear_due'
  | 'set_priority'
  | 'add_tag'
  | 'remove_tag'
  | 'set_human_only'
  | 'add_comment'
  | 'move_to_column'

/** 写时间动作的模式：只在为空时写，或者总是覆盖 */
export type TimeMode = 'if_empty' | 'always'

/** 规则里的一个动作，每种动作只用到其中一两个字段 */
export interface RuleAction {
  type: ActionType
  mode?: TimeMode
  tag?: string
  text?: string
  column_id?: number
  days?: number
  priority?: number
  flag?: boolean
}

export interface Rule {
  id: number
  board_id: number
  name: string
  enabled: boolean
  trigger: TriggerType
  /** 触发所在的列，0 表示任意列 */
  column_id: number
  /** 卡片筛选语法写的条件，空表示无条件 */
  condition: string
  actions: RuleAction[]
  sort: number
  created_at: string
  updated_at: string
}

/** 看板加上它的列和规则 */
export interface BoardDetail extends Board {
  columns: Column[]
  rules: Rule[]
}

export interface Card {
  id: number
  board_id: number
  column_id: number
  number: number
  /** 卡片编号，例如 SLATE-12 */
  key: string
  title: string
  description: string
  position: number
  /** 0 未设置，1 低，2 中，3 高 */
  priority: number
  tags: string[]
  human_only: boolean
  /** 依赖的卡片主键 */
  waits_on: number[]
  due_at: string | null
  started_at: string | null
  finished_at: string | null
  /** 当前持有这张卡的 agent 名称，空串表示没有 */
  claimed_by: string
  claim_expires_at: string | null
  branch: string
  url: string
  /** 乐观锁版本号 */
  version: number
  created_by: string
  updated_by: string
  created_at: string
  updated_at: string
}

/** 卡片加上服务端从列、依赖、租约推出来的只读信息 */
export interface CardView extends Card {
  /** 所在列的名称 */
  column: string
  /** 在完成列里 */
  done: boolean
  /** 有还没完成的依赖 */
  waiting: boolean
  /** 正被 agent 领取，并且租约没到期 */
  claim_active: boolean
}

export interface Comment {
  id: number
  board_id: number
  card_id: number
  author_kind: ActorKind
  author: string
  body: string
  created_at: string
  updated_at: string
}

/** 依赖列表里一张卡片的概要 */
export interface CardRef {
  id: number
  key: string
  title: string
  column: string
  done: boolean
}

export interface CardDetail extends CardView {
  /** 所在列给 agent 的工作指令 */
  column_instruction: string
  dependencies: CardRef[]
  comments: Comment[]
}

/** 整板快照：看板、列、规则、全部卡片，加上读取数据之前的最新事件序号 */
export interface Snapshot extends BoardDetail {
  cards: CardView[]
  seq: number
}

/** 筛选语法查询的结果 */
export interface CardList {
  cards: CardView[]
  total: number
  truncated: boolean
}

/** 事件日志里的一条事件，载荷的字段见 kanban.event.go.md */
export interface BoardEvent {
  id: number
  board_id: number
  card_id: number
  type: string
  actor_kind: ActorKind
  actor: string
  payload: Record<string, unknown>
  created_at: string
}

export interface EventPage {
  events: BoardEvent[]
  latest: number
  has_more: boolean
  /** 为真表示起点之后有事件已被清理，应重新读整块看板 */
  truncated: boolean
}

/** 活动流里出现过的一个操作者 */
export interface ActivityActor {
  kind: ActorKind
  name: string
}

/** 活动流的一页：事件从新到旧；latest 和 actors 不受筛选条件影响 */
export interface ActivityPage {
  events: BoardEvent[]
  /** 这一页之后还有更早的、满足条件的事件 */
  has_more: boolean
  latest: number
  actors: ActivityActor[]
}

/** 读活动流的参数，都可以不填 */
export interface ActivityQuery {
  /** 只看序号比它小的事件，翻页用 */
  before?: number
  limit?: number
  /** 事件类型，多个取并集 */
  types?: readonly string[]
  actor_kind?: ActorKind
  actor?: string
  /** 只看这张卡片的事件 */
  card_id?: number
}

/** 接入说明里一段可以直接复制的命令或配置 */
export interface Snippet {
  /** 用途：`worker`（工作视图的 MCP）、`manager`（管理视图的 MCP）、`skill`（安装 skill）；网页按它选各语言的小标题 */
  kind: string
  /** 服务端写的中文说明，网页不认识 kind 时才用它兜底 */
  title: string
  lang: 'sh' | 'json'
  code: string
}

export interface GuideHost {
  id: string
  name: string
  mcp: Snippet[]
  skill: Snippet[]
}

/** /api/v1/connect 返回的接入说明；每个 host 的 mcp 里依次是工作视图和管理视图的命令 */
export interface ConnectGuide {
  base_url: string
  mcp_url: string
  install_url: string
  skill_url: string
  hosts: GuideHost[]
}

// ---- 请求入参 ----

export type BoardTemplate = 'dev' | 'simple' | 'empty'

export interface CreateBoardInput {
  name: string
  slug?: string
  description?: string
  template?: BoardTemplate
  locale?: string
}

export interface UpdateBoardInput {
  name?: string
  description?: string
}

export interface CreateColumnInput {
  name: string
  description?: string
  instruction?: string
  agent_pickup?: boolean
  is_done?: boolean
  next_column_id?: number
}

export interface UpdateColumnInput {
  name?: string
  description?: string
  instruction?: string
  agent_pickup?: boolean
  is_done?: boolean
  /** 填 0 表示清除 */
  next_column_id?: number
}

export interface CreateCardInput {
  title: string
  description?: string
  column_id?: number
  priority?: number
  tags?: string[]
  human_only?: boolean
  waits_on?: number[]
  due_at?: string
  started_at?: string
  finished_at?: string
  branch?: string
  url?: string
}

/** 只提交出现的字段；标签和依赖传空数组表示清空，时间传空串表示清除 */
export interface UpdateCardInput {
  title?: string
  description?: string
  priority?: number
  tags?: string[]
  human_only?: boolean
  waits_on?: number[]
  due_at?: string
  started_at?: string
  finished_at?: string
  branch?: string
  url?: string
  /** 上次读到的版本号，不一致服务端返回 409 */
  version?: number
}

/** column_id 为 0 表示留在当前列；before_id 和 after_id 最多填一个 */
export interface MoveCardInput {
  column_id?: number
  before_id?: number
  after_id?: number
}

export interface CreateRuleInput {
  name: string
  enabled?: boolean
  trigger: TriggerType
  column_id?: number
  condition?: string
  actions: RuleAction[]
}

export interface UpdateRuleInput {
  name?: string
  enabled?: boolean
  trigger?: TriggerType
  column_id?: number
  condition?: string
  actions?: RuleAction[]
}

/** 服务端统一的错误体 */
export interface ErrorBody {
  reason?: string
  msg?: string
  trace_id?: string
}

/** 访问口令的状态：required 表示服务端启用了口令；authenticated 表示当前浏览器是否已通过 */
export interface AuthStatus {
  required: boolean
  authenticated: boolean
}
