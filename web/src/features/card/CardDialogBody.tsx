// 卡片弹窗正文（对齐 Trello 的两栏卡片）：
// 宽屏左栏是标题、一排属性按钮（含依赖、分支、链接）、描述、时间信息，右栏是浅灰底的「评论和活动」，两栏各自滚动；
// 窄屏单栏，DOM 顺序就是显示顺序：标题 → 属性 → 描述 → 时间信息 → 评论和活动。

import { useTranslation } from '@/i18n/react'
import type { CardDetail, Engineer, UpdateCardInput } from '@/lib/api/types'
import { useBoardSnapshot } from '@/features/board/useBoard'
import { formatDateTime } from '@/lib/format'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { CARD_TITLE_MAX } from '@/lib/limits'
import { CardMeta } from './CardMeta'
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

/** 快照还没读到时用的空名册，引用固定避免重复渲染 */
const NO_ENGINEERS: Engineer[] = []

interface CardDialogBodyProps {
  card: CardDetail
  depOptions: CardDetail['dependencies']
  lang: string
  boardId: number
  onPatch: (input: UpdateCardInput) => void
  onDesc: (description: string) => void
  patchBusy: boolean
  onRelease: () => void
  releaseBusy: boolean
}

/** 被 agent 领取时的横幅：谁在做、租约到什么时候，人可以释放 */
function ClaimBanner({ card, engineers, lang, onRelease, busy }: { card: CardDetail; engineers: Engineer[]; lang: string; onRelease: () => void; busy: boolean }) {
  const { t } = useTranslation()
  if (!card.claim_active || !card.claimed_by) return null
  const engineer = engineers.find((e) => e.id === card.claim_engineer_id)
  // 保留每种语言自己的日期位置，只为时间插入单独的字体节点。
  const [beforeTime, afterTime] = t('card.claim.banner', { name: card.claimed_by }).split(/\{\{\s*time\s*\}\}/)
  return (
    <div className="flex flex-col gap-2.5 rounded-2xl bg-agent/10 px-4 py-3 text-ui sm:flex-row sm:items-center sm:justify-between">
      <span>
        {beforeTime}
        <span className="font-mono">{formatDateTime(card.claim_expires_at, lang)}</span>
        {afterTime}
        {engineer && ` · ${t('card.claim.engineer', { name: engineer.name })}`}
      </span>
      <Button type="button" size="sm" variant="secondary" disabled={busy} onClick={onRelease}>
        {t('card.claim.release')}
      </Button>
    </div>
  )
}

/** 两栏布局主体 */
export function CardDialogBody({ card, depOptions, lang, boardId, onPatch, onDesc, patchBusy, onRelease, releaseBusy }: CardDialogBodyProps) {
  const { t } = useTranslation()
  const version = card.version
  const engineers = useBoardSnapshot(boardId).data?.engineers ?? NO_ENGINEERS
  const patch = (p: Record<string, unknown>) => onPatch(p as UpdateCardInput)

  return (
    <div className="flex min-h-0 flex-1 flex-col md:flex-row">
      <div className="min-w-0 space-y-5 px-5 py-5 sm:px-6 md:flex-1 md:overflow-y-auto">
        <ClaimBanner card={card} engineers={engineers} lang={lang} onRelease={onRelease} busy={releaseBusy} />
        <div>
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
        <CardMeta card={card} depOptions={depOptions} engineers={engineers} onPatch={patch} />
        <section>
          <DescriptionEditor key={card.version} value={card.description} onSave={onDesc} busy={patchBusy} boardId={boardId} />
        </section>
        <CardTimeMeta card={card} lang={lang} />
      </div>
      <aside className="min-w-0 border-t border-border/60 bg-secondary/50 px-5 py-5 md:w-[21rem] md:shrink-0 md:overflow-y-auto md:border-t-0 md:border-l">
        <CardTimeline cardId={card.id} boardId={boardId} comments={card.comments} />
      </aside>
    </div>
  )
}
