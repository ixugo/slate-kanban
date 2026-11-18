// 设置页「列」：列表、排序、增删改。

import { ArrowDownIcon, ArrowUpIcon, PencilIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from '@/i18n/react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { CardView, Column } from '@/lib/api/types'
import { useErrorToast } from '@/lib/useErrorToast'
import { ColumnDialog } from './ColumnDialog'
import { DeleteColumnDialog } from './DeleteColumnDialog'
import { useSettingsMutations } from './useSettingsMutations'
import { useTemplateText } from '@/lib/useTemplateText'
import { countCardsInColumn, moveColumnDown, moveColumnUp } from './validation'

interface Props {
  boardId: number
  columns: Column[]
  cards: CardView[]
}

/** 列管理区块 */
export function ColumnsSection({ boardId, columns, cards }: Props) {
  const { t } = useTranslation()
  const toastError = useErrorToast()
  const { reorderColumns } = useSettingsMutations(boardId)
  const [editor, setEditor] = useState<Column | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Column | null>(null)
  const { columnName } = useTemplateText()
  const sorted = [...columns].sort((a, b) => a.sort - b.sort || a.id - b.id)

  const move = (columnId: number, dir: 'up' | 'down') => {
    const ids = dir === 'up' ? moveColumnUp(columns, columnId) : moveColumnDown(columns, columnId)
    if (!ids) return
    reorderColumns.mutate(ids, { onError: toastError })
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-ui text-muted-foreground">{t('settings.columns.subtitle')}</p>
        <Button size="sm" onClick={() => setEditor('new')}>
          <PlusIcon />
          {t('settings.columns.add')}
        </Button>
      </div>
      <ul className="divide-y divide-border overflow-hidden rounded-2xl bg-card shadow-card">
        {sorted.map((col) => (
          <li key={col.id} className="flex flex-wrap items-center gap-2 px-4 py-3.5">
            <div className="min-w-0 flex-1">
              <p className="truncate font-medium">{columnName(col.name)}</p>
              <p className="text-xs text-muted-foreground">
                {t('settings.columns.cardCount', { count: countCardsInColumn(col.id, cards) })}
              </p>
              <div className="mt-1 flex flex-wrap gap-1">
                {col.agent_pickup && <Badge variant="secondary">{t('board.columnAi')}</Badge>}
                {col.is_done && <Badge variant="outline">{t('board.columnDone')}</Badge>}
              </div>
            </div>
            <div className="flex shrink-0 gap-1">
              <Button type="button" size="icon-sm" variant="ghost" aria-label={t('settings.columns.moveUp')} onClick={() => move(col.id, 'up')}>
                <ArrowUpIcon />
              </Button>
              <Button type="button" size="icon-sm" variant="ghost" aria-label={t('settings.columns.moveDown')} onClick={() => move(col.id, 'down')}>
                <ArrowDownIcon />
              </Button>
              <Button type="button" size="icon-sm" variant="ghost" aria-label={t('common.edit')} onClick={() => setEditor(col)}>
                <PencilIcon />
              </Button>
              <Button type="button" size="icon-sm" variant="ghost" aria-label={t('common.delete')} onClick={() => setDeleting(col)}>
                <Trash2Icon />
              </Button>
            </div>
          </li>
        ))}
      </ul>
      <ColumnDialog
        key={editor === 'new' ? 'new' : editor?.id ?? 'closed'}
        boardId={boardId}
        columns={columns}
        column={editor && editor !== 'new' ? editor : undefined}
        open={editor !== null}
        onOpenChange={(v) => !v && setEditor(null)}
      />
      {deleting && (
        <DeleteColumnDialog
          boardId={boardId}
          column={deleting}
          columns={columns}
          cards={cards}
          open
          onOpenChange={(v) => !v && setDeleting(null)}
        />
      )}
    </div>
  )
}