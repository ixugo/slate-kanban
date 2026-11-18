// 卡片弹窗正文：宽屏左栏是标题、描述、属性，右栏是「评论和活动」（滚动时固定在视野里）；窄屏单栏，顺序是标题、描述、属性、评论和活动。

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
import { CardTimeline } from './CardTimeline'

/** 让标题输入框的高度跟着内容走，长标题换行后也能完整看到（不依赖浏览器的 field-sizing） */
function fitTitleHeight(el: HTMLTextAreaElement | null) {
  if (!el) return
  el.style.height = 'auto'
  // border-box 的高度要把上下边框也算进去，否则内容会多出几像素
  const border = el.offsetHeight - el.clientHeight
  el.style.height = `${el.scrollHeight + border}px`
}

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

/** 两栏布局主体；DOM 顺序就是窄屏的显示顺序：标题 → 描述 → 属性 → 评论和活动 → 底部信息 */
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

  return (
    <div className="grid gap-x-8 gap-y-6 md:grid-cols-[minmax(0,1fr)_21rem] md:grid-rows-[auto_auto_auto_1fr] md:items-start">
      <div className="min-w-0 space-y-3 md:col-start-1 md:row-start-1">
        {card.claim_active && card.claimed_by ? (
          <div className="flex flex-col gap-2.5 rounded-2xl bg-agent/10 px-4 py-3 text-ui sm:flex-row sm:items-center sm:justify-between">
            <span>{t('card.claim.banner', { name: card.claimed_by, time: formatDateTime(card.claim_expires_at, lang) })}</span>
            <Button type="button" size="sm" variant="secondary" disabled={releaseBusy} onClick={onRelease}>
              {t('card.claim.release')}
            </Button>
          </div>
        ) : null}
        <Label htmlFor="card-title" className="sr-only">
          {t('card.field.title')}
        </Label>
        <textarea
          id="card-title"
          ref={fitTitleHeight}
          rows={1}
          defaultValue={card.title}
          maxLength={CARD_TITLE_MAX}
          className="-mx-2.5 block w-[calc(100%+1.25rem)] resize-none overflow-hidden rounded-xl border border-transparent bg-transparent px-2.5 py-1.5 text-[1.5rem] leading-8 font-semibold tracking-tight outline-none transition-[background-color,border-color,box-shadow] duration-200 hover:bg-secondary/60 focus-visible:border-ring focus-visible:bg-field focus-visible:ring-[3px] focus-visible:ring-ring/25"
          aria-label={t('card.field.title')}
          onInput={(e) => fitTitleHeight(e.currentTarget)}
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
      <section className="min-w-0 md:col-start-1 md:row-start-2">
        <DescriptionEditor key={card.version} value={card.description} onSave={onDesc} busy={patchBusy} />
      </section>
      <section className="min-w-0 md:col-start-1 md:row-start-3">
        <CardProperties
          card={card}
          columns={columns}
          depOptions={depOptions}
          version={version}
          onPatch={(p) => onPatch(p as UpdateCardInput)}
          onColumn={onColumn}
        />
      </section>
      <div className="min-w-0 md:col-start-2 md:row-span-4 md:row-start-1 md:max-h-[calc(90dvh-7rem)] md:self-start md:overflow-y-auto md:sticky md:top-0 md:pr-1">
        <CardTimeline cardId={card.id} boardId={boardId} comments={card.comments} />
      </div>
      <footer className="min-w-0 space-y-3 md:col-start-1 md:row-start-4">
        {card.column_instruction ? (
          <details className="rounded-xl bg-secondary/60 p-3 text-xs">
            <summary className="cursor-pointer font-medium">{t('card.columnInstruction')}</summary>
            <p className="mt-2 text-muted-foreground">{card.column_instruction}</p>
          </details>
        ) : null}
        <CardTimeMeta card={card} lang={lang} />
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button type="button" variant="destructive" size="sm" className="w-full sm:w-auto">
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
      </footer>
    </div>
  )
}
