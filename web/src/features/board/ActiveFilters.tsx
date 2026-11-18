// 工具栏下方的「当前筛选」一行：每个选中的条件一个小标签，点叉取消，最右边可以一键清除全部。

import { XIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import type { Column } from '@/lib/api/types'
import { useLocalizedColumns } from '@/lib/useTemplateText'
import { describeFilters } from './filterChips'
import { EMPTY_FILTERS, type BoardFilters } from './filters'

interface ActiveFiltersProps {
  filters: BoardFilters
  columns: readonly Column[]
  onChange: (next: BoardFilters) => void
}

/** 当前筛选条件一览；什么都没选时不占位置 */
export function ActiveFilters({ filters, columns, onChange }: ActiveFiltersProps) {
  const { t } = useTranslation()
  const localized = useLocalizedColumns(columns)
  const chips = describeFilters(filters, localized, t)
  if (chips.length === 0) return null
  return (
    <div role="group" aria-label={t('filter.active')} className="flex shrink-0 flex-wrap items-center gap-1.5 px-4 pb-2 sm:px-6">
      {chips.map((chip) => (
        <button
          key={chip.key}
          type="button"
          aria-label={t('filter.remove', { name: chip.label })}
          onClick={() => onChange(chip.next)}
          className="inline-flex h-6 items-center gap-1 rounded-full bg-secondary pr-1.5 pl-2.5 text-xs font-medium text-foreground/80 transition-colors duration-200 outline-none hover:bg-accent focus-visible:ring-[3px] focus-visible:ring-ring/40"
        >
          <span className="max-w-[16rem] truncate">{chip.label}</span>
          <XIcon className="size-3 text-muted-foreground" aria-hidden />
        </button>
      ))}
      <Button type="button" variant="ghost" size="xs" onClick={() => onChange(EMPTY_FILTERS)}>
        {t('filter.clearAll')}
      </Button>
    </div>
  )
}
