// 起步模板（研发流程、简单）在五种语言下的列名、列说明和默认规则名。
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
    'zh-CN': '想法池',
    'zh-TW': '想法池',
    en: 'Ideas',
    ja: 'アイデア',
    ko: '아이디어',
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

/** 起步模板里各列给人看的说明 */
const COLUMN_DESCRIPTIONS: Record<ColumnKind, Record<Lang, string>> = {
  idea: {
    'zh-CN': '还没想清楚的想法和需求，由人整理后再决定要不要做。',
    'zh-TW': '還沒想清楚的想法與需求，由人整理後再決定要不要做。',
    en: 'Rough ideas and requests that are not ready yet. A person sorts them out and decides whether to do them.',
    ja: 'まだ固まっていないアイデアや要望。人が整理して、やるかどうかを決めます。',
    ko: '아직 정리되지 않은 아이디어와 요청입니다. 사람이 정리한 뒤 진행할지 결정합니다.',
  },
  todo: {
    'zh-CN': '确定要做、正在排队的任务。',
    'zh-TW': '確定要做、正在排隊的任務。',
    en: 'Tasks that are confirmed and waiting in line.',
    ja: 'やることが決まっていて、順番待ちのタスク。',
    ko: '하기로 확정되어 순서를 기다리는 작업입니다.',
  },
  doing: {
    'zh-CN': '正在做的任务。卡片被拖进这一列后，agent 会来领取并开始工作。',
    'zh-TW': '正在做的任務。卡片被拖進這一列後，agent 會來領取並開始工作。',
    en: 'Tasks being worked on. Once a card is dragged into this column, an agent picks it up and starts working.',
    ja: '作業中のタスク。カードがこの列にドラッグされると、エージェントが担当して作業を始めます。',
    ko: '작업 중인 카드입니다. 카드가 이 열로 이동하면 에이전트가 맡아서 작업을 시작합니다.',
  },
  review: {
    'zh-CN': 'agent 已经做完、等人检查的任务。',
    'zh-TW': 'agent 已經做完、等人檢查的任務。',
    en: 'Tasks an agent has finished and a person needs to check.',
    ja: 'エージェントが終えて、人の確認を待つタスク。',
    ko: '에이전트가 마치고 사람의 확인을 기다리는 작업입니다.',
  },
  done: {
    'zh-CN': '已经做完并验收的任务。',
    'zh-TW': '已經做完並驗收的任務。',
    en: 'Tasks that are finished and accepted.',
    ja: '終わって確認も済んだタスク。',
    ko: '끝나고 확인까지 마친 작업입니다.',
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
const DESCRIPTION_KIND = reverse(COLUMN_DESCRIPTIONS)
const RULE_KIND = reverse(RULE_NAMES)

/** 列名：是起步文案就换成 lang 的说法，否则原样返回 */
export function localizeColumnName(name: string, lang: Lang): string {
  const kind = NAME_KIND.get(name)
  return kind ? COLUMN_NAMES[kind][lang] : name
}

/** 列说明：规则同列名 */
export function localizeColumnDescription(description: string, lang: Lang): string {
  const kind = DESCRIPTION_KIND.get(description)
  return kind ? COLUMN_DESCRIPTIONS[kind][lang] : description
}

/** 默认规则名：规则同列名 */
export function localizeRuleName(name: string, lang: Lang): string {
  const kind = RULE_KIND.get(name)
  return kind ? RULE_NAMES[kind][lang] : name
}
