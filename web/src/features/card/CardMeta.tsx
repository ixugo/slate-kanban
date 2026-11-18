// 卡片弹窗标题下面的一排属性：优先级、标签、截止、依赖、分支、链接、仅人工，做成紧凑的小按钮（对齐 Trello 的「添加到卡片」）。
// 没设值时按钮上只有图标和名称；设了值就把值显示在按钮上。点开按钮才出现编辑控件，平时不占地方。

import { useState, type ReactNode } from 'react'
import { CheckIcon, ClockIcon, FlagIcon, GitBranchIcon, Link2Icon, LinkIcon, LockIcon, TagIcon, XIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { useLanguage } from '@/i18n/useLanguage'
import type { CardDetail } from '@/lib/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { formatDateTime, isPast, toLocalInput } from '@/lib/format'
import { CARD_BRANCH_MAX, CARD_URL_MAX } from '@/lib/limits'
import { toast } from '@/lib/toast'
import { cn } from '@/lib/utils'
import { DependencyPicker } from './DependencyPicker'
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

/** 标签按钮上最多直接写出几枚标签，多出来的折成「+N」 */
const CHIP_TAGS_SHOWN = 2

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

/** 标签：按钮上直接列出前几枚带颜色的标签，点开后在标签库里勾选、编辑 */
function TagsChip({ card, onPatch }: CardMetaProps) {
  const { t } = useTranslation()
  const tags = card.tags ?? []
  const rest = tags.length - CHIP_TAGS_SHOWN
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button type="button" variant="secondary" aria-label={tags.length > 0 ? tags.join('、') : undefined} className={cn(CHIP_CLASS, tags.length > 0 && 'bg-accent')}>
          <ChipLabel icon={<TagIcon />}>
            {tags.length > 0 ? (
              <span className="flex items-center gap-1">
                {tags.slice(0, CHIP_TAGS_SHOWN).map((tag) => (
                  <TagBadge key={tag} name={tag} className="max-w-[6rem]" />
                ))}
                {rest > 0 && <span className="text-xs text-muted-foreground">+{rest}</span>}
              </span>
            ) : (
              t('card.field.tags')
            )}
          </ChipLabel>
        </Button>
      </PopoverTrigger>
      {/* 编辑页（30 色色板）比较高：高度不超过窗口剩余空间，放不下时弹层内部滚动，保存按钮始终够得着 */}
      <PopoverContent align="start" collisionPadding={8} className="max-h-(--radix-popover-content-available-height) w-72 overflow-y-auto">
        <LabelPicker tags={tags} onChange={(next) => onPatch({ tags: next, version: card.version })} />
      </PopoverContent>
    </Popover>
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
          <ChipLabel icon={<ClockIcon />}>{card.due_at ? formatDateTime(card.due_at, lang) : t('card.field.due')}</ChipLabel>
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
export function CardMeta({ card, depOptions, onPatch }: CardMetaProps & { depOptions: CardDetail['dependencies'] }) {
  const { t } = useTranslation()

  const saveUrl = (url: string) => {
    const norm = normalizeCardUrl(url)
    if (!norm.ok) {
      toast.error(t('card.urlInvalid'))
      return
    }
    onPatch({ url: norm.value.slice(0, CARD_URL_MAX), version: card.version })
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      <PriorityChip card={card} onPatch={onPatch} />
      <TagsChip card={card} onPatch={onPatch} />
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
  )
}
