// 看板工具栏搜索框：防抖查询、结果统计、语法错误提示。

import { SearchIcon } from 'lucide-react'
import { forwardRef, useId } from 'react'
import { useTranslation } from '@/i18n/react'
import { FILTER_MAX, SEARCH_LIMIT } from '@/lib/limits'
import { SearchHelp } from './SearchHelp'

interface SearchBoxProps {
  query: string
  onChange: (q: string) => void
  matched: number
  total: number
  truncated: boolean
  syntaxError: string
  searching: boolean
  active: boolean
}

/** 筛选语法搜索输入框 */
export const SearchBox = forwardRef<HTMLInputElement, SearchBoxProps>(function SearchBox(
  { query, onChange, matched, total, truncated, syntaxError, searching, active },
  ref,
) {
  const { t } = useTranslation()
  const errId = useId()
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-1 sm:max-w-md">
      <div className="flex items-center gap-1">
        <div className="relative min-w-0 flex-1">
          <SearchIcon
            className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden
          />
          <input
            ref={ref}
            value={query}
            onChange={(e) => onChange(e.target.value.slice(0, FILTER_MAX))}
            placeholder={t('search.placeholder')}
            aria-invalid={syntaxError ? true : undefined}
            aria-describedby={syntaxError ? errId : undefined}
            className="h-9 w-full rounded-full border border-input bg-field pr-4 pl-9 text-base transition-[border-color,box-shadow] duration-200 outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/25 aria-invalid:border-destructive aria-invalid:ring-[3px] aria-invalid:ring-destructive/20 md:text-ui"
          />
        </div>
        <SearchHelp onExample={onChange} />
      </div>
      {active && !syntaxError && (
        <p className="text-[11px] text-muted-foreground">
          {matched === 0 ? t('search.noResults') : t('board.columnCount', { matched, total })}
          {truncated ? ` · ${t('search.truncated', { limit: SEARCH_LIMIT })}` : ''}
          {searching ? ` · ${t('common.loading')}` : ''}
        </p>
      )}
      {syntaxError && (
        <p id={errId} role="alert" className="text-[11px] text-destructive">
          {syntaxError}
        </p>
      )}
    </div>
  )
})