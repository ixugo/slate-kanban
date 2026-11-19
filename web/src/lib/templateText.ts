// 起步模板（研发流程、简单）在五种语言下的列名和默认规则名。
// 内容和服务端 internal/core/kanban/board_template.go 里的 templateTexts 逐字一致（那边有测试核对这份文件）。
// 看板上存的是创建时那种语言的文字，网页显示时如果发现它正好是某个起步文案，就换成当前界面语言的说法；
// 用户自己起的名字、改过的文字都原样显示，只需要一种语言。

import type { Lang } from '@/i18n/languages'

/** 起步模板里的五种列 */
export type ColumnKind = 'idea' | 'todo' | 'doing' | 'review' | 'done'

/** 起步模板里的三条默认规则 */
export type RuleKind = 'start' | 'finish' | 'reopen'

/** 起步模板里各列的名称 */
const COLUMN_NAMES: Record<ColumnKind, Record<Lang, string>> = {
  idea: {
    'zh-CN': '收件箱',
    'zh-TW': '收件匣',
    en: 'Inbox',
    ja: '受信トレイ',
    ko: '받은 편지함',
  },
  todo: {
    'zh-CN': '待办',
    'zh-TW': '待辦',
    en: 'To do',
    ja: '未着手',
    ko: '할 일',
  },
  doing: {
    'zh-CN': '进行中',
    'zh-TW': '進行中',
    en: 'In progress',
    ja: '進行中',
    ko: '진행 중',
  },
  review: {
    'zh-CN': '审查',
    'zh-TW': '審查',
    en: 'Review',
    ja: 'レビュー',
    ko: '검토',
  },
  done: {
    'zh-CN': '完成',
    'zh-TW': '完成',
    en: 'Done',
    ja: '完了',
    ko: '완료',
  },
}

/** 起步模板里三条默认规则的名称：开始计时、完成计时、重新打开 */
const RULE_NAMES: Record<RuleKind, Record<Lang, string>> = {
  start: {
    'zh-CN': '开始计时',
    'zh-TW': '開始計時',
    en: 'Start timer',
    ja: '開始時刻を記録',
    ko: '시작 시간 기록',
  },
  finish: {
    'zh-CN': '完成计时',
    'zh-TW': '完成計時',
    en: 'Finish timer',
    ja: '完了時刻を記録',
    ko: '완료 시간 기록',
  },
  reopen: {
    'zh-CN': '重新打开',
    'zh-TW': '重新開啟',
    en: 'Reopen',
    ja: '再オープン',
    ko: '다시 열기',
  },
}

/** 把「任一语言的文案 → 种类」做成反查表 */
function reverse<K extends string>(table: Record<K, Record<Lang, string>>): Map<string, K> {
  const map = new Map<string, K>()
  for (const kind of Object.keys(table) as K[]) {
    for (const text of Object.values(table[kind])) map.set(text, kind)
  }
  return map
}

const NAME_KIND = reverse(COLUMN_NAMES)
const RULE_KIND = reverse(RULE_NAMES)

/** 列名：是起步文案就换成 lang 的说法，否则原样返回 */
export function localizeColumnName(name: string, lang: Lang): string {
  const kind = NAME_KIND.get(name)
  return kind ? COLUMN_NAMES[kind][lang] : name
}

/** 默认规则名：规则同列名 */
export function localizeRuleName(name: string, lang: Lang): string {
  const kind = RULE_KIND.get(name)
  return kind ? RULE_NAMES[kind][lang] : name
}
