// 看板页顶栏：名称、搜索与筛选、活动、设置、实时状态。

import type { RefObject } from 'react'
import { Link } from 'react-router-dom'
import { ActivityIcon, ArchiveIcon, EllipsisIcon, LayoutGridIcon, ListIcon, SettingsIcon } from 'lucide-react'
import { DropdownMenu } from 'radix-ui'
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
  noResults: boolean
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
  onActivityOpen: () => void
  onArchiveOpen: () => void
}

/** 看板画布上方的工具栏 */
export function BoardToolbar({
  board,
  searchRef,
  query,
  onQueryChange,
  noResults,
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
  onArchiveOpen,
}: BoardToolbarProps) {
  const { t } = useTranslation()
  const menuItemClass = 'flex cursor-default items-center gap-2 rounded-lg px-2.5 py-2 outline-none data-highlighted:bg-accent [&_svg]:size-4 [&_svg]:shrink-0'
  return (
    <form
      className="flex shrink-0 flex-col gap-3 px-4 pt-4 pb-3 sm:flex-row sm:items-start sm:px-6"
      onSubmit={(e) => e.preventDefault()}
    >
      <div className="flex min-h-9 min-w-0 items-center gap-2.5">
        <h1 className="truncate text-title font-semibold tracking-tight">{board.name}</h1>
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
          noResults={noResults}
          truncated={truncated}
          syntaxError={syntaxError}
          searching={searching}
          active={searchActive}
          actions={(
            <>
              <FilterPopover {...filter} />
              <DropdownMenu.Root modal={false}>
                <DropdownMenu.Trigger asChild>
                  <Button type="button" variant="outline" size="icon" className="size-9 border-input bg-field" aria-label={t('common.more')}>
                    <EllipsisIcon aria-hidden />
                  </Button>
                </DropdownMenu.Trigger>
                <DropdownMenu.Portal>
                  <DropdownMenu.Content
                    align="end"
                    sideOffset={4}
                    collisionPadding={12}
                    className="z-50 min-w-36 rounded-xl material-panel p-1 text-ui text-popover-foreground"
                  >
                    <DropdownMenu.Item asChild className={menuItemClass}>
                      <Link to={`/b/${board.id}/settings`}>
                        <SettingsIcon aria-hidden />
                        {t('board.settings')}
                      </Link>
                    </DropdownMenu.Item>
                    <DropdownMenu.Item className={menuItemClass} onSelect={onActivityOpen}>
                      <ActivityIcon aria-hidden />
                      {t('activity.open')}
                    </DropdownMenu.Item>
                    <DropdownMenu.Item className={menuItemClass} onSelect={onArchiveOpen}>
                      <ArchiveIcon aria-hidden />
                      {t('card.archive.title')}
                    </DropdownMenu.Item>
                  </DropdownMenu.Content>
                </DropdownMenu.Portal>
              </DropdownMenu.Root>
            </>
          )}
        />
        <LiveStatus state={liveState} onRetry={onLiveRetry} gone={gone} />
      </div>
    </form>
  )
}
