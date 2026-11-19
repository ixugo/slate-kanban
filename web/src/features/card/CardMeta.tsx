// 卡片弹窗标题下面的一排属性：优先级、标签、截止、依赖、分支、链接、仅人工，做成紧凑的小按钮（对齐 Trello 的「添加到卡片」）。
// 没设值时按钮上只有图标和名称；设了值就把值显示在按钮上。点开按钮才出现编辑控件，平时不占地方。
// 标签例外：设了标签以后单独占一行，显示成大号标签（对齐 Trello）。

import { useState, type ReactNode } from 'react'
import { CheckIcon, ClockIcon, FlagIcon, GitBranchIcon, Link2Icon, LinkIcon, LockIcon, PlusIcon, TagIcon, XIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { useLanguage } from '@/i18n/useLanguage'
import type { CardDetail, Engineer } from '@/lib/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { formatDateTime, isPast, toLocalInput } from '@/lib/format'
import { CARD_BRANCH_MAX, CARD_URL_MAX } from '@/lib/limits'
import { toast } from '@/lib/toast'
import { cn } from '@/lib/utils'
import { DependencyPicker } from './DependencyPicker'
import { EngineerChip } from './EngineerChip'
import { LabelPicker } from '@/features/labels/LabelPicker'
import { TagBadge } from '@/features/labels/TagBadge'
import { normalizeCardUrl } from './validateUrl'

interface CardMetaProps {
  card: CardDetail
  onPatch: (patch: Record<string, unknown>) => void
}

interface TextChipProps {
  icon: ReactNode
  /** 没有值时按钮上的名称，也是输入框的读屏名称 */
  name: string
  value: string
  max: number
  /** 关闭弹层时值变了才调用，参数是输入框里的原文 */
  onCommit: (next: string) => void
  /** 按钮上显示的文字；不给就直接显示值 */
  display?: (value: string) => string
}

/** 优先级的取值顺序，下标就是服务端的数值 */
const PRIORITIES = ['none', 'low', 'medium', 'high'] as const

/** 属性小按钮的外观：圆角矩形、浅底；设了值（active）时底色更深 */
const CHIP_CLASS = 'h-8 max-w-full rounded-lg px-2.5 font-medium'

/** 一个属性小按钮：图标加文字，作为弹层的触发器或开关 */
function ChipLabel({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <>
      {icon}
      <span className="truncate">{children}</span>
    </>
  )
}

/** 优先级：点开后在四个档位里选一个 */
function PriorityChip({ card, onPatch }: CardMetaProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button type="button" variant="secondary" className={cn(CHIP_CLASS, card.priority > 0 && 'bg-accent')}>
          <ChipLabel icon={<FlagIcon />}>
            {card.priority > 0 ? `${t('card.field.priority')}: ${t(`board.priority.${PRIORITIES[card.priority]}`)}` : t('card.field.priority')}
          </ChipLabel>
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-48 gap-0.5 p-1.5">
        <div role="radiogroup" aria-label={t('card.field.priority')} className="flex flex-col gap-0.5">
          {PRIORITIES.map((p, i) => (
            <Button
              key={p}
              type="button"
              role="radio"
              aria-checked={card.priority === i}
              variant="ghost"
              className="h-8 justify-between rounded-lg px-2.5"
              onClick={() => {
                if (card.priority !== i) onPatch({ priority: i, version: card.version })
                setOpen(false)
              }}
            >
              {t(`board.priority.${p}`)}
              {card.priority === i && <CheckIcon />}
            </Button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}

/** 没有标签时放在属性按钮一排里的「标签」按钮；点开是标签选择器 */
function TagsChip() {
  const { t } = useTranslation()
  return (
    <PopoverTrigger asChild>
      <Button type="button" variant="secondary" className={CHIP_CLASS}>
        <ChipLabel icon={<TagIcon />}>{t('card.field.tags')}</ChipLabel>
      </Button>
    </PopoverTrigger>
  )
}

/** 设了标签以后单独占一行（对齐 Trello）：小标题加一排大号标签，末尾的「+」点开标签选择器 */
function TagsSection({ tagIds }: { tagIds: number[] }) {
  const { t } = useTranslation()
  return (
    <section aria-label={t('card.field.tags')} className="space-y-1.5">
      <h3 className="text-xs font-semibold text-muted-foreground">{t('card.field.tags')}</h3>
      <div className="flex flex-wrap items-center gap-1.5">
        {tagIds.map((id) => (
          <TagBadge key={id} id={id} size="lg" />
        ))}
        <PopoverTrigger asChild>
          <Button type="button" variant="secondary" size="icon" className="size-8 rounded-md" aria-label={t('card.field.tags')}>
            <PlusIcon />
          </Button>
        </PopoverTrigger>
      </div>
    </section>
  )
}

/** 截止时间：点开后选日期和时间，关闭弹层时保存；逾期的显示成红色 */
function DueChip({ card, onPatch }: CardMetaProps) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState('')
  const overdue = Boolean(card.due_at) && !card.done && isPast(card.due_at)

  const onOpenChange = (next: boolean) => {
    if (next) setDraft(toLocalInput(card.due_at))
    // 关闭时只在真的改过才保存，空串表示清除
    else if (draft !== toLocalInput(card.due_at)) onPatch({ due_at: draft, version: card.version })
    setOpen(next)
  }

  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger asChild>
        <Button type="button" variant="secondary" className={cn(CHIP_CLASS, card.due_at && 'bg-accent', overdue && 'text-destructive')}>
          <ChipLabel icon={<ClockIcon />}>{card.due_at ? <span className="font-mono">{formatDateTime(card.due_at, lang)}</span> : t('card.field.due')}</ChipLabel>
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-64">
        <Input
          type="datetime-local"
          aria-label={t('card.field.due')}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && onOpenChange(false)}
        />
        <Button type="button" variant="ghost" size="sm" className="self-start" disabled={draft === ''} onClick={() => setDraft('')}>
          <XIcon />
          {t('card.dueClear')}
        </Button>
      </PopoverContent>
    </Popover>
  )
}

/** 依赖：按钮上显示依赖的张数，点开后增删 */
function DepsChip({ card, depOptions, onPatch }: CardMetaProps & { depOptions: CardDetail['dependencies'] }) {
  const { t } = useTranslation()
  const count = (card.waits_on ?? []).length
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button type="button" variant="secondary" className={cn(CHIP_CLASS, count > 0 && 'bg-accent')}>
          <ChipLabel icon={<Link2Icon />}>{count > 0 ? `${t('card.field.deps')} ${count}` : t('card.field.deps')}</ChipLabel>
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80">
        <DependencyPicker
          cardId={card.id}
          selected={card.waits_on}
          options={depOptions}
          onChange={(waits_on) => onPatch({ waits_on, version: card.version })}
        />
      </PopoverContent>
    </Popover>
  )
}

/** 一行文字的属性（分支、链接）：点开后输入，关闭弹层时保存 */
function TextChip({ icon, name, value, max, onCommit, display }: TextChipProps) {
  const [draft, setDraft] = useState(value)
  const [open, setOpen] = useState(false)

  const onOpenChange = (next: boolean) => {
    if (next) setDraft(value)
    else if (draft !== value) onCommit(draft)
    setOpen(next)
  }

  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger asChild>
        <Button type="button" variant="secondary" className={cn(CHIP_CLASS, 'max-w-56', value && 'bg-accent')}>
          <ChipLabel icon={icon}>{value ? (display ? display(value) : value) : name}</ChipLabel>
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-72">
        <Input
          aria-label={name}
          value={draft}
          maxLength={max}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && onOpenChange(false)}
        />
      </PopoverContent>
    </Popover>
  )
}

/** 链接按钮上只显示网址的主机名，完整地址在输入框里 */
function hostOf(url: string): string {
  try {
    return new URL(url).host || url
  } catch {
    return url
  }
}

/** 标题下面的一排属性按钮，放不下就换行 */
export function CardMeta({ card, depOptions, engineers, onPatch }: CardMetaProps & { depOptions: CardDetail['dependencies']; engineers: Engineer[] }) {
  const { t } = useTranslation()
  // 标签选择器的开关放在这里：加第一枚标签时入口从按钮挪到标签区，弹层不能跟着关掉
  const [tagsOpen, setTagsOpen] = useState(false)
  const tagIds = card.tag_ids ?? []
  const hasTags = tagIds.length > 0

  const saveUrl = (url: string) => {
    const norm = normalizeCardUrl(url)
    if (!norm.ok) {
      toast.error(t('card.urlInvalid'))
      return
    }
    onPatch({ url: norm.value.slice(0, CARD_URL_MAX), version: card.version })
  }

  return (
    <Popover open={tagsOpen} onOpenChange={setTagsOpen}>
      <div className="flex flex-wrap items-center gap-2">
        <PriorityChip card={card} onPatch={onPatch} />
        {!hasTags && <TagsChip />}
        <EngineerChip card={card} engineers={engineers} onPatch={onPatch} />
        <DueChip card={card} onPatch={onPatch} />
        <DepsChip card={card} depOptions={depOptions} onPatch={onPatch} />
        <TextChip
          icon={<GitBranchIcon />}
          name={t('card.field.branch')}
          value={card.branch}
          max={CARD_BRANCH_MAX}
          onCommit={(next) => onPatch({ branch: next.trim().slice(0, CARD_BRANCH_MAX), version: card.version })}
        />
        <TextChip icon={<LinkIcon />} name={t('card.field.url')} value={card.url} max={CARD_URL_MAX} onCommit={saveUrl} display={hostOf} />
        <Button
          type="button"
          variant="secondary"
          aria-pressed={card.human_only}
          className={cn(CHIP_CLASS, card.human_only && 'bg-primary text-primary-foreground hover:bg-primary/88')}
          onClick={() => onPatch({ human_only: !card.human_only, version: card.version })}
        >
          <ChipLabel icon={<LockIcon />}>{t('card.field.humanOnly')}</ChipLabel>
        </Button>
      </div>
      {hasTags && <TagsSection tagIds={tagIds} />}
      {/* 标签选择器：编辑页（30 色色板）比较高，高度不超过窗口剩余空间，放不下时弹层内部滚动，保存按钮始终够得着 */}
      <PopoverContent align="start" collisionPadding={8} className="max-h-(--radix-popover-content-available-height) w-72 overflow-y-auto">
        <LabelPicker tagIds={tagIds} onChange={(next) => onPatch({ tag_ids: next, version: card.version })} />
      </PopoverContent>
    </Popover>
  )
}
