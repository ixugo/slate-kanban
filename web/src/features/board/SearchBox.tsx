// 看板工具栏搜索框：防抖查询、结果统计、语法错误提示；后面可以接上「筛选」按钮。

import { SearchIcon, XIcon } from 'lucide-react'
import { forwardRef, useId, useRef, type ReactNode } from 'react'
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
  /** 放在搜索框和帮助按钮后面的操作，比如「筛选」按钮 */
  actions?: ReactNode
}

/** 筛选语法搜索输入框 */
export const SearchBox = forwardRef<HTMLInputElement, SearchBoxProps>(function SearchBox(
  { query, onChange, matched, total, truncated, syntaxError, searching, active, actions },
  ref,
) {
  const { t } = useTranslation()
  const errId = useId()
  const inputRef = useRef<HTMLInputElement | null>(null)
  /** 同时把输入框交给外部的 ref（快捷键 / 要聚焦它）和本组件自己的 ref（清空后要把焦点放回去） */
  const setRefs = (el: HTMLInputElement | null) => {
    inputRef.current = el
    if (typeof ref === 'function') ref(el)
    else if (ref) ref.current = el
  }
  /** 清空搜索文字，并把焦点放回输入框方便继续输入 */
  const clear = () => {
    onChange('')
    inputRef.current?.focus()
  }
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-1 sm:max-w-md">
      <div className="flex items-center gap-1">
        <div className="relative min-w-0 flex-1">
          <SearchIcon
            className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden
          />
          <input
            ref={setRefs}
            value={query}
            onChange={(e) => onChange(e.target.value.slice(0, FILTER_MAX))}
            placeholder={t('search.placeholder')}
            aria-invalid={syntaxError ? true : undefined}
            aria-describedby={syntaxError ? errId : undefined}
            className="h-9 w-full rounded-full border border-input bg-field pr-9 pl-9 text-base transition-[border-color,box-shadow] duration-200 outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/25 aria-invalid:border-destructive aria-invalid:ring-[3px] aria-invalid:ring-destructive/20 md:text-ui"
          />
          {query && (
            <button
              type="button"
              onClick={clear}
              aria-label={t('search.clear')}
              title={t('search.clear')}
              className="absolute top-1/2 right-2 flex size-5 -translate-y-1/2 items-center justify-center rounded-full text-muted-foreground transition-colors outline-none hover:bg-secondary hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/40"
            >
              <XIcon className="size-3.5" aria-hidden />
            </button>
          )}
        </div>
        <SearchHelp onExample={onChange} />
        {actions}
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