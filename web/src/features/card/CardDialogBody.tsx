// 卡片弹窗正文：宽屏两栏，窄屏单栏。

import { useTranslation } from '@/i18n/react'
import type { CardDetail, Column, UpdateCardInput } from '@/lib/api/types'
import { formatDateTime } from '@/lib/format'
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
    <aside className="flex flex-col gap-4 md:col-start-2 md:row-span-4 md:row-start-1 md:self-start">
      {card.claim_active && card.claimed_by ? (
        <div className="flex flex-col gap-2.5 rounded-2xl bg-agent/10 px-4 py-3 text-ui">
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
        <details className="rounded-xl bg-secondary/60 p-3 text-xs">
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
    <div className="grid gap-6 md:grid-cols-[1fr_17.5rem] md:grid-rows-[auto_auto_auto_1fr] md:items-start">
      <div className="space-y-1">
        <Label htmlFor="card-title" className="sr-only">
          {t('card.field.title')}
        </Label>
        <textarea
          id="card-title"
          rows={1}
          defaultValue={card.title}
          maxLength={CARD_TITLE_MAX}
          className="-mx-2.5 block w-[calc(100%+1.25rem)] resize-none rounded-xl border border-transparent bg-transparent px-2.5 py-1.5 text-[1.5rem] leading-8 font-semibold tracking-tight [field-sizing:content] outline-none transition-[background-color,border-color,box-shadow] duration-200 hover:bg-secondary/60 focus-visible:border-ring focus-visible:bg-field focus-visible:ring-[3px] focus-visible:ring-ring/25"
          aria-label={t('card.field.title')}
          onKeyDown={(e) => {
            // 标题是单行文字：回车提交（失焦时保存），不插入换行
            if (e.key === 'Enter') {
              e.preventDefault()
              e.currentTarget.blur()
            }
          }}
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