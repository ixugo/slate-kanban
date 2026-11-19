// 规则列表里一行白话摘要：when + 可选 condition + 动作列表。

import { useTranslation } from '@/i18n/react'
import type { TranslationKey } from '@/i18n/types'
import type { Column, Rule } from '@/lib/api/types'
import { buildRuleSummary, RULE_SUMMARY_THEN_KEY } from './ruleSummary'

interface Props {
  rule: Rule
  columns: Column[]
}

/** 渲染一条规则的自然语言摘要 */
export function RuleSummaryText({ rule, columns }: Props) {
  const { t } = useTranslation()
  const parts = buildRuleSummary(rule, columns)
  const when = t(parts.whenKey as TranslationKey, parts.whenParams)
  const condition = parts.conditionKey
    ? t(parts.conditionKey as TranslationKey, { text: rule.condition.trim() })
    : ''
  const actions = parts.actions.map((a) => t(a.key as TranslationKey, a.params)).join(t('settings.rules.summary.join'))
  const body = condition ? `${when} ${condition} ${t(RULE_SUMMARY_THEN_KEY)} ${actions}` : `${when} ${t(RULE_SUMMARY_THEN_KEY)} ${actions}`
  return <span className="text-muted-foreground">{body}</span>
}