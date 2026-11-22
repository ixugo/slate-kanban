// 任务「日期」：开始日期和截止日期，点开弹窗用勾选框加日期（截止还可选时刻），点「保存」才提交，「取消」放弃。
// 没设日期时是属性按钮一排里的「日期」按钮；设了以后挪到标签区下面单独一块，按钮上显示具体日期。

import { ClockIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from '@/i18n/react'
import { useLanguage } from '@/i18n/useLanguage'
import type { CardDetail } from '@/lib/api/types'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { formatDate, formatDateTime, isPast } from '@/lib/format'
import { cn } from '@/lib/utils'
import { datesPatch, draftOf, draftProblem, withDefaults, type DatesDraft } from './datesDraft'

interface CardDatesProps {
  card: CardDetail
  onPatch: (patch: Record<string, unknown>) => void
}

/** 属性小按钮的外观，和 CardMeta 里的其他按钮保持一致 */
const CHIP_CLASS = 'h-8 max-w-full rounded-lg px-2.5 font-medium'

/** 按钮上显示的日期文字：开始、截止都有时写成“开始 – 截止” */
function summary(card: CardDetail, lang: string, startedLabel: string): string {
  const start = card.started_at ? formatDate(card.started_at, lang) : ''
  const due = card.due_at ? formatDateTime(card.due_at, lang) : ''
  if (start && due) return `${start} – ${due}`
  return due || `${startedLabel} ${start}`
}

/** 一行：勾选框加名称，勾选后才能填日期 */
function DateRow({ label, checked, onChecked, children }: { label: string; checked: boolean; onChecked: (v: boolean) => void; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label className="text-xs font-semibold text-muted-foreground">{label}</Label>
      <div className="flex items-center gap-2">
        <Checkbox checked={checked} onCheckedChange={(v) => onChecked(v === true)} aria-label={label} />
        {children}
      </div>
    </div>
  )
}

/** 弹窗里的表单：草稿只存在这里，保存才交给 onPatch，取消或点外面就丢掉 */
function DatesForm({ card, onPatch, onDone }: CardDatesProps & { onDone: () => void }) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<DatesDraft>(() => draftOf(card))
  const set = (patch: Partial<DatesDraft>) => setDraft((d) => withDefaults({ ...d, ...patch }))
  const problem = draftProblem(draft)

  const save = () => {
    const patch = datesPatch(card, draft)
    if (Object.keys(patch).length > 0) onPatch({ ...patch, version: card.version })
    onDone()
  }

  return (
    <div className="space-y-3">
      <DateRow label={t('card.field.startDate')} checked={draft.start} onChecked={(start) => set({ start })}>
        <Input
          type="date"
          className="font-mono"
          aria-label={t('card.field.startDate')}
          disabled={!draft.start}
          value={draft.startDate}
          onChange={(e) => set({ startDate: e.target.value })}
        />
      </DateRow>
      <DateRow label={t('card.field.due')} checked={draft.due} onChecked={(due) => set({ due })}>
        <Input
          type="date"
          className="font-mono"
          aria-label={t('card.field.due')}
          disabled={!draft.due}
          value={draft.dueDate}
          onChange={(e) => set({ dueDate: e.target.value })}
        />
        <Input
          type="time"
          className="w-28 font-mono"
          aria-label={t('card.dueTime')}
          disabled={!draft.due}
          value={draft.dueTime}
          onChange={(e) => set({ dueTime: e.target.value })}
        />
      </DateRow>
      {problem === 'order' && (
        <p role="alert" className="text-xs text-destructive">
          {t('card.datesOrder')}
        </p>
      )}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" size="sm" onClick={onDone}>
          {t('common.cancel')}
        </Button>
        <Button type="button" size="sm" disabled={problem !== null} onClick={save}>
          {t('common.save')}
        </Button>
      </div>
    </div>
  )
}

/** 日期弹窗：触发按钮由调用方给（没设日期时是一排里的小按钮，设了以后是日期区里的按钮） */
function DatesPopover({ card, onPatch, className, children }: CardDatesProps & { className?: string; children: React.ReactNode }) {
  const [open, setOpen] = useState(false)
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button type="button" variant="secondary" className={cn(CHIP_CLASS, className)}>
          {children}
        </Button>
      </PopoverTrigger>
      {/* 每次打开都重新挂载表单，草稿从任务当前值开始 */}
      <PopoverContent align="start" className="w-72">
        {open && <DatesForm card={card} onPatch={onPatch} onDone={() => setOpen(false)} />}
      </PopoverContent>
    </Popover>
  )
}

/** 没设日期时放在属性按钮一排里的「日期」按钮 */
export function DatesChip({ card, onPatch }: CardDatesProps) {
  const { t } = useTranslation()
  return (
    <DatesPopover card={card} onPatch={onPatch}>
      <ClockIcon />
      <span className="truncate">{t('card.field.dates')}</span>
    </DatesPopover>
  )
}

/** 设了日期以后显示在标签区下面：小标题加一个能自动换行的容器，里面是显示具体日期的按钮；逾期的截止日期标红 */
export function DatesSection({ card, onPatch }: CardDatesProps) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const overdue = Boolean(card.due_at) && !card.done && isPast(card.due_at)
  return (
    <section aria-label={t('card.field.dates')} className="space-y-1.5">
      <h3 className="text-xs font-semibold text-muted-foreground">{t('card.field.dates')}</h3>
      <div className="flex flex-wrap items-center gap-1.5">
        <DatesPopover card={card} onPatch={onPatch} className={cn('bg-accent', overdue && 'text-destructive')}>
          <ClockIcon />
          <span className="truncate font-mono">{summary(card, lang, t('card.field.started'))}</span>
        </DatesPopover>
      </div>
    </section>
  )
}
