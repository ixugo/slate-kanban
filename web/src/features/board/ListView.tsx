// 列表视图：表格展示卡片，遵守搜索可见集，不支持拖拽；数据量在几百张以内不虚拟滚动。

import { useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { PlusIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { useLanguage } from '@/i18n/useLanguage'
import type { CardView, Column } from '@/lib/api/types'
import { formatDate, formatDateTime, isPast } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useOnlineStatus } from '@/pwa/useOnlineStatus'
import { TagBadge } from '@/features/labels/TagBadge'
import { Button } from '@/components/ui/button'
import { useTemplateText } from '@/lib/useTemplateText'
import { AddCard } from './AddCard'
import {
  DEFAULT_SORT,
  ariaSortFor,
  nextSortState,
  sortCards,
  type SortKey,
  type SortState,
} from './sortCards'
import { displayKey } from '@/lib/cardKey'

/** 标签列最多展示枚数，与磁贴一致 */
const TAGS_VISIBLE = 3

/** 优先级圆点颜色 */
const PRIORITY_DOT = ['', 'bg-p-low', 'bg-p-mid', 'bg-p-high'] as const

interface ListViewProps {
  columns: Column[]
  cards: CardView[]
  /** 非 null 时只展示其中的卡片 id */
  visibleIds: ReadonlySet<number> | null
  listAddOpen: boolean
  onListAddOpen: () => void
  onListAddClose: () => void
  onAddCard: (title: string) => void
  addBusy?: boolean
}

/** 表头排序按钮：放在 th 里，带 aria-sort */
function SortHeader({
  label,
  sortKey,
  state,
  onSort,
}: {
  label: string
  sortKey: SortKey
  state: SortState
  onSort: (key: SortKey) => void
}) {
  const sort = ariaSortFor(sortKey, state)
  return (
    <th scope="col" aria-sort={sort} className="whitespace-nowrap px-3 py-2 text-left font-medium">
      <button
        type="button"
        className="inline-flex items-center gap-1 rounded-md px-1 py-0.5 text-[0.8125rem] font-medium text-foreground hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        onClick={() => onSort(sortKey)}
      >
        {label}
        {sort !== 'none' && (
          <span className="text-[0.625rem] text-muted-foreground" aria-hidden>
            {sort === 'ascending' ? '↑' : '↓'}
          </span>
        )}
      </button>
    </th>
  )
}

/** 列表视图主组件 */
export default function ListView({
  columns,
  cards,
  visibleIds,
  listAddOpen,
  onListAddOpen,
  onListAddClose,
  onAddCard,
  addBusy,
}: ListViewProps) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const { columnName: localizeName } = useTemplateText()
  const online = useOnlineStatus()
  const [params] = useSearchParams()
  const [sort, setSort] = useState<SortState>(DEFAULT_SORT)

  const visible = useMemo(() => {
    const base = visibleIds === null ? cards : cards.filter((c) => visibleIds.has(c.id))
    return sortCards(base, columns, sort)
  }, [cards, columns, visibleIds, sort])

  const columnName = (card: CardView) => localizeName(card.column || columns.find((c) => c.id === card.column_id)?.name || '')

  const priorityLabel = (p: number) => {
    if (p === 1) return t('board.priority.low')
    if (p === 2) return t('board.priority.medium')
    if (p === 3) return t('board.priority.high')
    return t('board.priority.none')
  }

  const onSort = (key: SortKey) => setSort((prev) => nextSortState(prev, key))

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-hidden px-4 pb-4 sm:px-6">
      <div className="shrink-0">
        {!online ? null : listAddOpen ? (
          <AddCard
            open
            onOpen={onListAddOpen}
            onClose={onListAddClose}
            onSubmit={onAddCard}
            busy={addBusy}
          />
        ) : (
          <Button type="button" variant="outline" size="sm" onClick={onListAddOpen}>
            <PlusIcon />
            {t('board.newCard')}
          </Button>
        )}
      </div>
      <div className="min-h-0 flex-1 overflow-x-auto overflow-y-auto rounded-xl border bg-card shadow-card">
        <table className="w-full min-w-[44rem] border-collapse text-[0.8125rem]">
          <caption className="sr-only">{t('list.caption')}</caption>
          <thead className="sticky top-0 z-[1] border-b bg-card/95 backdrop-blur-sm">
            <tr className="text-muted-foreground">
              <SortHeader label={t('list.colKey')} sortKey="key" state={sort} onSort={onSort} />
              <SortHeader label={t('list.colTitle')} sortKey="title" state={sort} onSort={onSort} />
              <SortHeader label={t('list.colColumn')} sortKey="column" state={sort} onSort={onSort} />
              <SortHeader label={t('list.colPriority')} sortKey="priority" state={sort} onSort={onSort} />
              <th scope="col" className="whitespace-nowrap px-3 py-2 text-left font-medium text-foreground">
                {t('list.colTags')}
              </th>
              <SortHeader label={t('list.colDue')} sortKey="due" state={sort} onSort={onSort} />
              <th scope="col" className="whitespace-nowrap px-3 py-2 text-left font-medium text-foreground">
                {t('list.colClaimed')}
              </th>
              <SortHeader label={t('list.colUpdated')} sortKey="updated" state={sort} onSort={onSort} />
            </tr>
          </thead>
          <tbody>
            {visible.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-3 py-8 text-center text-muted-foreground">
                  {t('list.empty')}
                </td>
              </tr>
            ) : (
              visible.map((card) => {
                const next = new URLSearchParams(params)
                next.set('card', String(card.id))
                const overdue = card.due_at && !card.done && isPast(card.due_at)
                return (
                  <tr
                    key={card.id}
                    className={cn('border-b border-border/60 hover:bg-muted/40', card.done && 'opacity-65')}
                  >
                    <td className="whitespace-nowrap px-3 py-2.5 font-mono text-muted-foreground">{displayKey(card.key)}</td>
                    <td className="min-w-[12rem] px-3 py-2.5">
                      <Link
                        to={{ search: next.toString() }}
                        className="font-medium text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        {card.title}
                      </Link>
                    </td>
                    <td className="whitespace-nowrap px-3 py-2.5 text-muted-foreground">{columnName(card)}</td>
                    <td className="whitespace-nowrap px-3 py-2.5">
                      <span className="inline-flex items-center gap-1.5">
                        {card.priority > 0 && (
                          <span
                            className={cn('size-1.5 rounded-full', PRIORITY_DOT[card.priority])}
                            aria-hidden
                          />
                        )}
                        {priorityLabel(card.priority)}
                      </span>
                    </td>
                    <td className="px-3 py-2.5">
                      <div className="flex flex-wrap items-center gap-1">
                        {(card.tag_ids ?? []).slice(0, TAGS_VISIBLE).map((id) => (
                          <TagBadge key={id} id={id} className="max-w-[6rem]" />
                        ))}
                        {(card.tag_ids ?? []).length > TAGS_VISIBLE && (
                          <span className="text-[0.6875rem] text-muted-foreground">
                            +{(card.tag_ids ?? []).length - TAGS_VISIBLE}
                          </span>
                        )}
                      </div>
                    </td>
                    <td
                      className={cn(
                        'whitespace-nowrap px-3 py-2.5 font-mono',
                        overdue ? 'font-medium text-destructive' : 'text-muted-foreground',
                      )}
                    >
                      {card.due_at ? formatDate(card.due_at, lang) : '—'}
                    </td>
                    <td className="whitespace-nowrap px-3 py-2.5 text-[0.75rem]">
                      {card.claim_active && card.claimed_by ? (
                        <span className="inline-flex items-center gap-1.5 font-medium text-agent">
                          <span className="signal-dot size-1.5 rounded-full bg-agent" aria-hidden />
                          {card.claimed_by}
                        </span>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </td>
                    <td className="whitespace-nowrap px-3 py-2.5 font-mono text-muted-foreground">
                      {formatDateTime(card.updated_at, lang)}
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}