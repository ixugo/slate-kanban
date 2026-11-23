// 看板工具栏搜索框：防抖查询、查询状态、语法错误提示；后面可以接上「筛选」按钮。

import { SearchIcon, XIcon } from 'lucide-react'
import { forwardRef, useId, useRef, type ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { useTranslation } from '@/i18n/react'
import { FILTER_MAX, SEARCH_LIMIT } from '@/lib/limits'
import { SearchHelp } from './SearchHelp'

interface SearchBoxProps {
  query: string
  onChange: (q: string) => void
  noResults: boolean
  truncated: boolean
  syntaxError: string
  searching: boolean
  active: boolean
  /** 放在搜索框后面的操作，比如「筛选」按钮 */
  actions?: ReactNode
}

/** 筛选语法搜索输入框 */
export const SearchBox = forwardRef<HTMLInputElement, SearchBoxProps>(function SearchBox(
  { query, onChange, noResults, truncated, syntaxError, searching, active, actions },
  ref,
) {
  const { t } = useTranslation()
  const errId = useId()
  const inputRef = useRef<HTMLInputElement | null>(null)
  const status = [
    noResults && t('search.noResults'),
    truncated && t('search.truncated', { limit: SEARCH_LIMIT }),
    searching && t('common.loading'),
  ].filter(Boolean).join(' · ')
  /** 同时把输入框交给外部的 ref（快捷键 / 要聚焦它）和本组件自己的 ref（清空后要把焦点放回去） */
  const setRefs = (el: HTMLInputElement | null) => {
    inputRef.current = el
    if (typeof ref === 'function') ref(el)
    else if (ref) ref.current = el
  }
  /** 清空或填入查询后，把焦点放回输入框方便继续输入 */
  const applyQuery = (next: string) => {
    onChange(next)
    inputRef.current?.focus()
  }
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-1 sm:flex-initial">
      <div className="flex items-center gap-1">
        <div className="relative w-[300px] min-w-0 max-w-[300px] flex-auto">
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
          <div className="absolute top-1/2 right-1 -translate-y-1/2">
            {query ? (
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                className="text-muted-foreground"
                onClick={() => applyQuery('')}
                aria-label={t('search.clear')}
                title={t('search.clear')}
              >
                <XIcon aria-hidden />
              </Button>
            ) : (
              <SearchHelp onExample={applyQuery} />
            )}
          </div>
        </div>
        {actions}
      </div>
      {active && !syntaxError && status && (
        <p className="text-[11px] text-muted-foreground">
          {status}
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
