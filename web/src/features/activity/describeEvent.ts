// 把一条事件翻成界面上的文字：服务端只存事件类型和载荷，各语言的句子在这里按类型拼出来。
// 事件类型和载荷字段见 .cursor/scopes/kanban.event.go.md。

import type { TFunction } from '@/i18n/react'
import type { TranslationKey } from '@/i18n/types'
import type { ActivityActor, BoardEvent, CardView } from '@/lib/api/types'

/** 自动化规则作为操作者时，名称的固定前缀，和服务端 actor.go 里的 automationActorPrefix 一致 */
const AUTOMATION_PREFIX = 'automation:'

/** 一条事件要显示的文字 */
export interface EventText {
  /** 一句话，不含操作者，例如「把卡片从「待办」移到「进行中」」 */
  text: string
  /** 补充说明（评论摘要、自动化执行了哪些动作），没有就是空串 */
  detail: string
  /** 这次修改了描述时，对应的变更记录编号；界面据此在句子后面加「查看变更」。旧事件和其他类型没有 */
  descriptionChangeId?: number
}

/** 事件涉及的卡片 */
export interface EventCard {
  /** 卡片还在看板上时的编号，用来做链接；已经被删除为 null */
  id: number | null
  key: string
  title: string
}

// 取出载荷对象；服务端存的内容万一不是合法 JSON 会退回成字符串，这时当成没有载荷。
function payloadOf(event: BoardEvent): Record<string, unknown> {
  const payload: unknown = event.payload
  return typeof payload === 'object' && payload !== null ? (payload as Record<string, unknown>) : {}
}

// 读载荷里的文字字段；缺失或类型不对返回空串。
function field(payload: Record<string, unknown>, key: string): string {
  const value = payload[key]
  return typeof value === 'string' ? value : ''
}

// 读载荷里的变更记录编号：必须是正整数，缺失或类型不对返回 undefined。
function changeId(payload: Record<string, unknown>, key: string): number | undefined {
  const value = payload[key]
  return typeof value === 'number' && Number.isInteger(value) && value > 0 ? value : undefined
}

// 读载荷里的文字数组，忽略不是文字的项。
function fields(payload: Record<string, unknown>, key: string): string[] {
  const value = payload[key]
  return Array.isArray(value) ? value.filter((v): v is string => typeof v === 'string') : []
}

// 卡片修改事件载荷里的字段名对应的界面文字：时间类用更完整的叫法，其余复用卡片详情里各属性的名称。
const CARD_FIELD_LABELS: Record<string, TranslationKey> = {
  title: 'card.field.title',
  description: 'card.field.description',
  priority: 'card.field.priority',
  tags: 'card.field.tags',
  human_only: 'card.field.humanOnly',
  waits_on: 'card.field.deps',
  due_at: 'activity.field.due_at',
  started_at: 'activity.field.started_at',
  finished_at: 'activity.field.finished_at',
  branch: 'card.field.branch',
  url: 'card.field.url',
}

// 卡片修改事件的句子：写明改了哪些属性，例如「修改了优先级、截止时间」；没有字段清单的旧事件退回成笼统的一句。
function cardUpdatedText(payload: Record<string, unknown>, t: TFunction): string {
  const labels = fields(payload, 'fields').map((name) => {
    const key = CARD_FIELD_LABELS[name]
    return key ? t(key) : name
  })
  if (labels.length === 0) return t('activity.event.cardUpdated')
  return t('activity.event.cardUpdatedFields', { fields: labels.join(t('settings.rules.autoName.join')) })
}

/** 操作者在界面上的名字：人叫「我」，规则触发显示成「自动化 · 规则名」，agent 用它自己报的名字 */
export function actorLabel(actor: ActivityActor, t: TFunction): string {
  if (actor.kind === 'human') return t('activity.actor.human')
  if (actor.kind === 'system') {
    const name = actor.name.startsWith(AUTOMATION_PREFIX) ? actor.name.slice(AUTOMATION_PREFIX.length) : actor.name
    return t('activity.actor.rule', { name })
  }
  return actor.name
}

// 规则动作的本地化名称；不认识的动作类型原样显示。
function actionName(type: string, t: TFunction): string {
  const key = `settings.rules.actionTypes.${type}`
  const label = t(key as TranslationKey)
  return label === key ? type : label
}

// 规则触发的补充说明：执行了哪些动作，有动作被跳过时再补一句。
function firedDetail(payload: Record<string, unknown>, t: TFunction): string {
  const names = fields(payload, 'actions').map((type) => actionName(type, t))
  const skipped = fields(payload, 'skipped').length
  return [names.join(t('settings.rules.autoName.join')), skipped > 0 ? t('activity.event.skipped', { count: skipped }) : '']
    .filter((part) => part !== '')
    .join(' · ')
}

// 卡片释放事件有三种原因，句子不一样：持有者主动释放、人收回、卡片换列。
function releasedText(payload: Record<string, unknown>, t: TFunction): string {
  const agent = field(payload, 'agent')
  switch (field(payload, 'why')) {
    case 'revoked':
      return t('activity.event.cardRevoked', { agent })
    case 'moved':
      return t('activity.event.cardClaimEnded', { agent })
    default:
      return t('activity.event.cardReleased')
  }
}

/** 一条事件的文字；不认识的事件类型（比如服务端比网页新）显示成一句通用的话，不会让列表出错 */
export function describeEvent(event: BoardEvent, t: TFunction): EventText {
  const p = payloadOf(event)
  const name = field(p, 'name')
  const plain = (text: string): EventText => ({ text, detail: '' })
  switch (event.type) {
    case 'board.created':
      return plain(t('activity.event.boardCreated'))
    case 'board.updated':
      return plain(t('activity.event.boardUpdated'))
    case 'board.labels_updated':
      return plain(t('activity.event.boardLabelsUpdated'))
    case 'column.created':
      return plain(t('activity.event.columnCreated', { name }))
    case 'column.updated':
      return plain(t('activity.event.columnUpdated', { name }))
    case 'column.deleted':
      return plain(t('activity.event.columnDeleted', { name }))
    case 'column.reordered':
      return plain(t('activity.event.columnReordered'))
    case 'card.created':
      return plain(t('activity.event.cardCreated', { column: field(p, 'column') }))
    case 'card.updated':
      return { text: cardUpdatedText(p, t), detail: '', descriptionChangeId: changeId(p, 'description_change_id') }
    case 'card.tags_updated':
      return plain(t('activity.event.cardTagsUpdated'))
    case 'card.moved':
      return plain(t('activity.event.cardMoved', { from: field(p, 'from'), to: field(p, 'to') }))
    case 'card.reordered':
      return plain(t('activity.event.cardReordered'))
    case 'card.deleted':
      return plain(t('activity.event.cardDeleted'))
    case 'card.claimed':
      return plain(t('activity.event.cardClaimed'))
    case 'card.released':
      return plain(releasedText(p, t))
    case 'comment.added':
      return { text: t('activity.event.commentAdded'), detail: field(p, 'snippet') }
    case 'comment.deleted':
      return plain(t('activity.event.commentDeleted'))
    case 'rule.created':
      return plain(t('activity.event.ruleCreated', { name }))
    case 'rule.updated':
      return plain(t('activity.event.ruleUpdated', { name }))
    case 'rule.deleted':
      return plain(t('activity.event.ruleDeleted', { name }))
    case 'automation.fired':
      return { text: t('activity.event.automationFired'), detail: firedDetail(p, t) }
    default:
      return plain(t('activity.event.unknown', { type: event.type }))
  }
}

/** 事件涉及的卡片：卡片还在就用它现在的编号和标题（改过名也显示新名字），已经没了就用事件里记下的 */
export function eventCard(event: BoardEvent, cardsById: ReadonlyMap<number, CardView>): EventCard | null {
  const live = cardsById.get(event.card_id)
  if (live) return { id: live.id, key: live.key, title: live.title }
  const p = payloadOf(event)
  const key = field(p, 'key')
  return key === '' ? null : { id: null, key, title: field(p, 'title') }
}
