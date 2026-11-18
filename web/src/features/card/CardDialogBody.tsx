// 卡片弹窗正文：宽屏两栏，窄屏单栏。

import { useTranslation } from '@/i18n/react'
import type { CardDetail, Column, UpdateCardInput } from '@/lib/api/types'
import { formatDateTime } from '@/lib/format'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { CARD_TITLE_MAX } from '@/lib/limits'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { CardProperties } from './CardProperties'
import { CardTimeMeta } from './CardTimeMeta'
import { DescriptionEditor } from './DescriptionEditor'
import { Comments } from './Comments'

interface CardDialogBodyProps {
  card: CardDetail
  columns: Column[]
  depOptions: CardDetail['dependencies']
  lang: string
  boardId: number
  onPatch: (input: UpdateCardInput) => void
  onColumn: (columnId: number) => void
  onDesc: (description: string) => void
  patchBusy: boolean
  onRelease: () => void
  releaseBusy: boolean
  onDelete: () => void
  deleteBusy: boolean
}

/** 两栏布局主体；DOM 顺序保证窄屏为标题→属性→描述→评论 */
export function CardDialogBody({
  card,
  columns,
  depOptions,
  lang,
  boardId,
  onPatch,
  onColumn,
  onDesc,
  patchBusy,
  onRelease,
  releaseBusy,
  onDelete,
  deleteBusy,
}: CardDialogBodyProps) {
  const { t } = useTranslation()
  const version = card.version

  const aside = (
    <aside className="flex flex-col gap-3 md:col-start-2 md:row-span-3 md:row-start-1 md:self-start">
      {card.claim_active && card.claimed_by ? (
        <div className="flex flex-col gap-2 rounded-md border border-signal/30 bg-signal/10 px-3 py-2 text-sm">
          <span>{t('card.claim.banner', { name: card.claimed_by, time: formatDateTime(card.claim_expires_at, lang) })}</span>
          <Button type="button" size="sm" variant="secondary" disabled={releaseBusy} onClick={onRelease}>
            {t('card.claim.release')}
          </Button>
        </div>
      ) : null}
      <CardProperties
        card={card}
        columns={columns}
        depOptions={depOptions}
        version={version}
        onPatch={(p) => onPatch(p as UpdateCardInput)}
        onColumn={onColumn}
      />
      {card.column_instruction ? (
        <details className="rounded-md border p-2 text-xs">
          <summary className="cursor-pointer font-medium">{t('card.columnInstruction')}</summary>
          <p className="mt-2 text-muted-foreground">{card.column_instruction}</p>
        </details>
      ) : null}
      <CardTimeMeta card={card} lang={lang} />
      <AlertDialog>
        <AlertDialogTrigger asChild>
          <Button type="button" variant="destructive" size="sm" className="w-full">
            {t('common.delete')}
          </Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('card.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('card.deleteBody')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction disabled={deleteBusy} onClick={onDelete}>
              {t('common.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </aside>
  )

  return (
    <div className="grid gap-4 md:grid-cols-[1fr_17.5rem] md:items-start">
      <div className="space-y-1">
        <Label htmlFor="card-title" className="sr-only">
          {t('card.field.title')}
        </Label>
        <Input
          id="card-title"
          defaultValue={card.title}
          maxLength={CARD_TITLE_MAX}
          className="border-0 px-0 text-lg font-semibold shadow-none focus-visible:ring-0"
          aria-label={t('card.field.title')}
          onBlur={(e) => {
            const title = e.target.value.trim().slice(0, CARD_TITLE_MAX)
            if (title !== card.title) onPatch({ title, version })
          }}
        />
      </div>
      {aside}
      <section className="min-w-0 md:col-start-1">
        <DescriptionEditor key={card.version} value={card.description} onSave={onDesc} busy={patchBusy} />
      </section>
      <section className="min-w-0 md:col-start-1">
        <Comments cardId={card.id} boardId={boardId} comments={card.comments} />
      </section>
    </div>
  )
}