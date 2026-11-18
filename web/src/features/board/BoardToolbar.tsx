// 看板页顶栏：名称、搜索与筛选、导出、设置、实时状态。

import type { RefObject } from 'react'
import { Link } from 'react-router-dom'
import { ChevronDownIcon, SettingsIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { API_PREFIX } from '@/lib/api/client'
import type { Board } from '@/lib/api/types'
import { FilterPopover, type FilterPopoverProps } from './FilterPopover'
import { SearchBox } from './SearchBox'
import { LiveStatus } from './LiveStatus'
import type { LiveState } from './useBoardLive'

interface BoardToolbarProps {
  board: Board
  searchRef: RefObject<HTMLInputElement | null>
  query: string
  onQueryChange: (q: string) => void
  matched: number
  total: number
  truncated: boolean
  syntaxError: string
  searching: boolean
  searchActive: boolean
  /** 「筛选」按钮和弹层的数据 */
  filter: FilterPopoverProps
  liveState: LiveState
  onLiveRetry: () => void
  gone?: boolean
}

/** 看板画布上方的工具栏 */
export function BoardToolbar({
  board,
  searchRef,
  query,
  onQueryChange,
  matched,
  total,
  truncated,
  syntaxError,
  searching,
  searchActive,
  filter,
  liveState,
  onLiveRetry,
  gone,
}: BoardToolbarProps) {
  const { t } = useTranslation()
  const exportBase = `${API_PREFIX}/boards/${board.id}/export`
  return (
    <form
      className="flex shrink-0 flex-col gap-3 px-4 pt-4 pb-3 sm:flex-row sm:items-start sm:px-6"
      onSubmit={(e) => e.preventDefault()}
    >
      <div className="flex min-h-9 min-w-0 items-center gap-2.5">
        <h1 className="truncate text-title font-semibold tracking-tight">{board.name}</h1>
        <span className="shrink-0 rounded-full bg-secondary px-2 py-0.5 font-mono text-[0.6875rem] text-muted-foreground">
          {board.slug}
        </span>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button type="button" variant="outline" size="sm">
              {t('board.export')}
              <ChevronDownIcon />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            <DropdownMenuItem asChild>
              <a href={`${exportBase}?format=json`} download>
                {t('board.exportJson')}
              </a>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <a href={`${exportBase}?format=md`} download>
                {t('board.exportMd')}
              </a>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <Button type="button" variant="ghost" size="sm" asChild>
          <Link to={`/b/${board.id}/settings`}>
            <SettingsIcon />
            <span className="hidden sm:inline">{t('board.settings')}</span>
          </Link>
        </Button>
      </div>
      <div className="flex min-w-0 flex-1 items-start gap-3 sm:justify-end">
        <SearchBox
          ref={searchRef}
          query={query}
          onChange={onQueryChange}
          matched={matched}
          total={total}
          truncated={truncated}
          syntaxError={syntaxError}
          searching={searching}
          active={searchActive}
          actions={<FilterPopover {...filter} />}
        />
        <LiveStatus state={liveState} onRetry={onLiveRetry} gone={gone} />
      </div>
    </form>
  )
}