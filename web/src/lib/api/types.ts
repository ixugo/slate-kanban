// 和后端 JSON 一一对应的类型，来源是 internal/core/kanban 下的 *.model.go、*.param.go，
// 以及 internal/web/api/connectapi。后端改字段时，这里要一起改。
// 时间一律是 RFC3339 字符串；可空的时间是 null。

/** 操作者类型：人、agent、系统（自动化规则） */
export type ActorKind = 'human' | 'agent' | 'system'

/** 看板标签库里的一项：一个颜色加一段文字，各自可改；文字可以为空，表示只有颜色。卡片靠主键引用标签 */
export interface Tag {
  id: number
  text: string
  /** 色相-深浅（1 最深，3 最浅），例如 green-2；空串表示默认灰色 */
  color: string
}

/** 工程师类型：dev 全栈、frontend 前端、backend 后端、ui UI 都做研发，review 测试做审查 */
export type EngineerKind = 'dev' | 'frontend' | 'backend' | 'ui' | 'review'

/** 看板工程师名册里的一位：每位是一个并发名额，prompt 是调度 agent 派子 agent 时用的角色提示词 */
export interface Engineer {
  id: number
  name: string
  kind: EngineerKind
  prompt: string
  /** 头像的随机种子，空表示按编号画 */
  avatar: string
}

export interface Board {
  id: number
  name: string
  /** 标签库：新看板自带 6 个标准色标签，可以改、可以加 */
  tags: Tag[]
  /** 工程师名册：新看板自带 2 个全栈和 1 个测试 */
  engineers: Engineer[]
  /** 卡片编号的前缀，创建后不能改 */
  slug: string
  description: string
  /** 看板关联的本地项目路径，agent 建 worktree 时用 */
  project_path: string
  created_at: string
  updated_at: string
  /** 最后活动时间：看板自身更新和卡片、评论等事件里最晚的一个，项目列表用它显示“更新于” */
  last_activity_at: string
}

export interface Column {
  id: number
  board_id: number
  name: string
  /** 给 agent 的指令：这一列的卡片该怎么处理 */
  instruction: string
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
  /** 卡片上的标签主键，指向看板的标签库（含没有文字的标签） */
  tag_ids: number[]
  human_only: boolean
  /** 依赖的卡片主键 */
  waits_on: number[]
  due_at: string | null
  started_at: string | null
  finished_at: string | null
  /** 当前持有这张卡的 agent 名称，空串表示没有 */
  claimed_by: string
  claim_expires_at: string | null
  /** 绑定或首次领取时分配的全栈工程师，0 表示还没有 */
  engineer_id: number
  /** 现在领取这张卡的工程师，0 表示没有 */
  claim_engineer_id: number
  branch: string
  url: string
  /** 乐观锁版本号 */
  version: number
  /** 未读动态数，仅看板磁贴展示 */
  unread_count: number
  created_by: string
  updated_by: string
  created_at: string
  updated_at: string
}

/** 卡片加上服务端从列、依赖、租约推出来的只读信息 */
export interface CardView extends Card {
  /** 卡片上带文字的标签的文字，给筛选选项和无障碍名称用；要颜色请用 tag_ids 查标签库 */
  tags: string[]
  /** 所在列的名称 */
  column: string
  /** 在完成列里 */
  done: boolean
  /** 有还没完成的依赖 */
  waiting: boolean
  /** 正被 agent 领取，并且租约没到期 */
  claim_active: boolean
}

/** 上传图片后的结果：url 可以直接写进 Markdown 的图片语法 */
export interface Attachment {
  token: string
  url: string
  content_type: string
  size: number
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

/** 卡片某个字段的一次修改记录：改之前、改之后的完整内容，用来左右对比（目前只记描述） */
export interface CardChange {
  id: number
  board_id: number
  card_id: number
  /** 被修改的字段名，目前只会是 description */
  field: string
  old_value: string
  new_value: string
  actor_kind: ActorKind
  actor: string
  created_at: string
  /** 同一张卡片里前一条、后一条变更记录的编号，没有就是 0 */
  prev_id: number
  next_id: number
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
  /** 不要这些类型的事件；和 types 重叠时以排除为准 */
  exclude_types?: readonly string[]
  actor_kind?: ActorKind
  actor?: string
  /** 只看这张卡片的事件 */
  card_id?: number
}

/** 接入说明里一段可以直接复制的命令或配置 */
export interface Snippet {
  /** 用途：`worker`（工作视图的 MCP）、`manager`（管理视图的 MCP）、`skill`（安装 skill）、`watch`（开长期会话）、`resume`（恢复会话）、`loop`（定时叫醒）；网页按它选各语言的小标题 */
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
  /** 长期挂着：开会话的命令，以及断了怎么恢复 */
  watch: Snippet[]
}

/** /api/v1/connect 返回的接入说明；每个 host 的 mcp 里依次是工作视图和管理视图的命令 */
export interface ConnectGuide {
  base_url: string
  mcp_url: string
  install_url: string
  skill_url: string
  hosts: GuideHost[]
  /** 用户唯一要贴的一行守候提示词；里面的 <看板名> 是占位符，网页按所选看板替换 */
  loop_prompt: string
  /** 长期挂着的注意事项，网页用各语言自己的文案，install.md 用它 */
  watch_notes: string[]
}

// ---- 请求入参 ----

export type BoardTemplate = 'dev' | 'simple' | 'empty'

export interface CreateBoardInput {
  name: string
  slug?: string
  description?: string
  template?: BoardTemplate
  locale?: string
  /** 非 0 时从该自定义项目模板复制结构，忽略 template */
  project_template_id?: number
}

/** 自定义项目模板列表项 */
export interface ProjectTemplateSummary {
  id: number
  name: string
  updated_at: string
}

/** 导入模板成功后的详情 */
export interface ProjectTemplateDetail extends ProjectTemplateSummary {
  tags: { text: string; color: string }[]
  engineers: { name: string; kind: EngineerKind; prompt: string; avatar?: string }[]
  columns: { name: string; instruction: string; is_done: boolean; next_column: number }[]
}

/** 修改标签库的一项：id 为空表示新增；整体替换，库里有而入参没有的算删除 */
export interface TagInput {
  /** 不填表示新增，由服务端分配 */
  id?: number
  text: string
  color: string
}

/** 修改工程师名册的一项：id 为空表示招募；整体替换，名册里有而入参没有的算解聘 */
export interface EngineerInput {
  id?: number
  name: string
  kind: EngineerKind
  prompt: string
  /** 头像随机种子；招募时留空由服务端生成，修改时留空沿用原来的 */
  avatar?: string
}

export interface UpdateBoardInput {
  name?: string
  description?: string
  /** 整体替换工程师名册：解聘的工程师会从所有卡片上摘掉 */
  engineers?: EngineerInput[]
  /** 整体替换标签库：库里原有、这里没有的算删除，顺序就是显示顺序 */
  tags?: TagInput[]
}

export interface CreateColumnInput {
  name: string
  instruction?: string
  is_done?: boolean
  next_column_id?: number
}

export interface UpdateColumnInput {
  name?: string
  instruction?: string
  is_done?: boolean
  /** 填 0 表示清除 */
  next_column_id?: number
}

export interface CreateCardInput {
  title: string
  description?: string
  column_id?: number
  priority?: number
  tag_ids?: number[]
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
  tag_ids?: number[]
  human_only?: boolean
  /** 绑定的全栈工程师，0 表示取消绑定 */
  engineer_id?: number
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

/** 服务端版本信息：version 是构建版本号（例如 v0.1.0），remark 是分支、提交和日期 */
export interface VersionInfo {
  version: string
  remark: string
}

/** 服务端告诉网页这次更新检查进行到了哪一步 */
export type UpdateCheckState = 'ok' | 'unreachable'

/** 自动更新的检查结果：所有构建都会检查；新 release 里当前平台的包上传完才算有更新；can_apply 为假时只能引导用户去 release 页面手动下载 */
export interface UpdateInfo {
  current: string
  latest: string
  /** 最新版本的更新说明（Markdown） */
  notes: string
  /** 最新 release 的页面地址 */
  url: string
  has_update: boolean
  can_apply: boolean
  /** 检查状态：区分开发版不查、GitHub 失败与正常查到版本 */
  check_state?: UpdateCheckState
}

/** 立即更新成功的结果：程序已被替换；restarting 为真表示服务端会自动重启，为假要用户手动重启 */
export interface UpdateApplied {
  version: string
  restarting: boolean
}

/** 访问口令的状态：required 表示服务端启用了口令；authenticated 表示当前浏览器是否已通过 */
export interface AuthStatus {
  required: boolean
  authenticated: boolean
}

/** 群机器人 webhook 类型 */
export type WebhookType = 'dingtalk' | 'wecom' | 'feishu'

/** 活动通知 webhook（密钥已脱敏） */
export interface WebhookView {
  id: number
  board_id: number
  name: string
  type: WebhookType
  url: string
  secret_set: boolean
  secret_mask: string
  enabled: boolean
  event_types: string[]
  actor_kinds: ActorKind[]
  move_column_ids: number[]
  created_at: string
  updated_at: string
}

export interface CreateWebhookInput {
  name: string
  type: WebhookType
  url: string
  secret?: string
  enabled?: boolean
  event_types?: string[]
  actor_kinds?: ActorKind[]
  move_column_ids?: number[]
}

export interface UpdateWebhookInput {
  name?: string
  type?: WebhookType
  url?: string
  secret?: string
  secret_clear?: boolean
  enabled?: boolean
  event_types?: string[]
  actor_kinds?: ActorKind[]
  move_column_ids?: number[]
}

export interface TestWebhookResult {
  ok: boolean
  message: string
}
