// 在组件里按当前界面语言显示起步模板的列名和默认规则名，用户自己写的文字不动。

import { useMemo } from 'react'
import { useLanguage } from '@/i18n/useLanguage'
import type { Column } from '@/lib/api/types'
import { localizeColumnName, localizeRuleName } from './templateText'

/** 返回按当前语言换文案的函数，语言不变时函数引用稳定 */
export function useTemplateText() {
  const { lang } = useLanguage()
  return useMemo(
    () => ({
      columnName: (name: string) => localizeColumnName(name, lang),
      ruleName: (name: string) => localizeRuleName(name, lang),
    }),
    [lang],
  )
}

/** 把一组列换成按当前语言显示的名称（只用于显示，别拿去提交保存） */
export function useLocalizedColumns(columns: readonly Column[]): Column[] {
  const { columnName } = useTemplateText()
  return useMemo(() => columns.map((c) => ({ ...c, name: columnName(c.name) })), [columns, columnName])
}
