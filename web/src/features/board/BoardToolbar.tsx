// 看板页顶栏：名称、搜索与筛选、活动、设置、实时状态。

import type { RefObject } from 'react'
import { Link } from 'react-router-dom'
import { ActivityIcon, LayoutGridIcon, ListIcon, SettingsIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import type { Board } from '@/lib/api/types'
import { FilterPopover, type FilterPopoverProps } from './FilterPopover'
import { SearchBox } from './SearchBox'
import { LiveStatus } from './LiveStatus'
import type { LiveState } from './useBoardLive'
import type { BoardViewMode } from './useBoardView'
import { cn } from '@/lib/utils'

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
  viewMode: BoardViewMode
  onViewModeChange: (mode: BoardViewMode) => void
  /** 点「活动」按钮：打开活动抽屉 */
  onActivityOpen: () => void
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
  viewMode,
  onViewModeChange,
  onActivityOpen,
}: BoardToolbarProps) {
  const { t } = useTranslation()
  return (
    <form
      className="flex shrink-0 flex-col gap-3 px-4 pt-4 pb-3 sm:flex-row sm:items-start sm:px-6"
      onSubmit={(e) => e.preventDefault()}
    >
      <div className="flex min-h-9 min-w-0 items-center gap-2.5">
        <h1 className="truncate text-title font-semibold tracking-tight">{board.name}</h1>
        <Button type="button" variant="ghost" size="sm" aria-label={t('activity.open')} onClick={onActivityOpen}>
          <ActivityIcon />
          <span className="hidden sm:inline">{t('activity.open')}</span>
        </Button>
        <Button type="button" variant="ghost" size="sm" asChild>
          <Link to={`/b/${board.id}/settings`}>
            <SettingsIcon />
            <span className="hidden sm:inline">{t('board.settings')}</span>
          </Link>
        </Button>
        <div
          className="flex shrink-0 rounded-lg border bg-muted/50 p-0.5"
          role="group"
          aria-label={t('board.viewToggle')}
        >
          <button
            type="button"
            aria-pressed={viewMode === 'board'}
            title={t('board.viewBoard')}
            className={cn(
              'inline-flex size-8 items-center justify-center rounded-md transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              viewMode === 'board' ? 'bg-background shadow-sm' : 'text-muted-foreground hover:text-foreground',
            )}
            onClick={() => onViewModeChange('board')}
          >
            <LayoutGridIcon className="size-4" aria-hidden />
            <span className="sr-only">{t('board.viewBoard')}</span>
          </button>
          <button
            type="button"
            aria-pressed={viewMode === 'list'}
            title={t('board.viewList')}
            className={cn(
              'inline-flex size-8 items-center justify-center rounded-md transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              viewMode === 'list' ? 'bg-background shadow-sm' : 'text-muted-foreground hover:text-foreground',
            )}
            onClick={() => onViewModeChange('list')}
          >
            <ListIcon className="size-4" aria-hidden />
            <span className="sr-only">{t('board.viewList')}</span>
          </button>
        </div>
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