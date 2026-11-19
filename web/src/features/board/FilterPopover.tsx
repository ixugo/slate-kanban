// 看板工具栏的「筛选」按钮和弹层（仿 Trello）：点选标签、列、状态、优先级、领取者、截止日期，
// 选择立刻生效；选好的条件会拼成和搜索框同一套筛选语法（见 filters.ts）。

import { ListFilterIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { useTemplateText } from '@/lib/useTemplateText'
import { Button } from '@/components/ui/button'
import { Chip, ChipGroup } from '@/components/ChipGroup'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import type { Column } from '@/lib/api/types'
import { cn } from '@/lib/utils'
import { tagText } from '@/features/labels/tagName'
import {
  DUE_CHOICES,
  EMPTY_FILTERS,
  PRIORITY_NAMES,
  STATE_FLAGS,
  countFilters,
  toggleClaimer,
  toggleColumn,
  toggleNoTags,
  togglePriority,
  toggleState,
  toggleTag,
  type BoardFilters,
  type TagMode,
  type TagOption,
} from './filters'

/** 弹层需要的数据和回调 */
export interface FilterPopoverProps {
  filters: BoardFilters
  onChange: (next: BoardFilters) => void
  columns: readonly Column[]
  /** 这块看板里出现过的标签 */
  tags: readonly TagOption[]
  /** 正在领取卡片的 agent 名称 */
  claimers: readonly string[]
}

/** 选了两个及以上标签时出现：在「任一」和「全部」之间切换 */
function TagModeSwitch({ mode, onChange }: { mode: TagMode; onChange: (mode: TagMode) => void }) {
  const { t } = useTranslation()
  return (
    <div
      role="radiogroup"
      aria-label={t('filter.tagMode.label')}
      className="inline-flex rounded-full bg-secondary p-0.5 text-xs"
    >
      {(['any', 'all'] as const).map((value) => (
        <button
          key={value}
          type="button"
          role="radio"
          aria-checked={mode === value}
          onClick={() => onChange(value)}
          className={cn(
            'rounded-full px-2.5 py-0.5 font-medium transition-colors duration-200 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/40',
            mode === value ? 'bg-card text-foreground shadow-card' : 'text-muted-foreground hover:text-foreground',
          )}
        >
          {t(`filter.tagMode.${value}`)}
        </button>
      ))}
    </div>
  )
}

/** 已选中、但当前看板里已经没有的取值也要留在选项里，否则没法取消 */
function withSelected<T>(options: readonly T[], selected: readonly T[], same: (a: T, b: T) => boolean): T[] {
  return [...options, ...selected.filter((s) => !options.some((o) => same(o, s)))]
}

/** 「筛选」按钮和它的弹层 */
export function FilterPopover({ filters, onChange, columns, tags, claimers }: FilterPopoverProps) {
  const { t } = useTranslation()
  const { columnName } = useTemplateText()
  const count = countFilters(filters)
  const tagOptions = withSelected(
    tags,
    filters.tags.map((name) => ({ name, count: 0 })),
    (a, b) => a.name.toLowerCase() === b.name.toLowerCase(),
  )
  const claimerOptions = withSelected(claimers, filters.claimers, (a, b) => a === b)

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-full border border-input bg-field px-3.5 text-ui font-medium text-foreground/80 transition-[background-color,border-color,box-shadow] duration-200 outline-none hover:bg-card focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/25 aria-expanded:bg-card"
        >
          <ListFilterIcon className="size-4" aria-hidden />
          {t('filter.button')}
          {count > 0 && (
            <span className="rounded-full bg-primary px-1.5 text-[0.6875rem] leading-4 font-semibold text-primary-foreground">
              {count}
            </span>
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        collisionPadding={12}
        className="overflow-y-auto p-4"
        style={{ width: 'min(23rem, calc(100vw - 2rem))', maxHeight: 'min(34rem, var(--radix-popover-content-available-height))' }}
      >
        <div className="flex flex-col gap-4">
          <div className="flex min-h-6 items-center justify-between gap-2">
            <p className="font-semibold">{t('filter.title')}</p>
            {count > 0 && (
              <Button type="button" variant="ghost" size="xs" onClick={() => onChange(EMPTY_FILTERS)}>
                {t('filter.clearAll')}
              </Button>
            )}
          </div>

          <ChipGroup
            title={t('filter.groups.tags')}
            aside={
              filters.tags.length > 1 ? (
                <TagModeSwitch mode={filters.tagMode} onChange={(tagMode) => onChange({ ...filters, tagMode })} />
              ) : null
            }
          >
            <Chip selected={filters.noTags} onClick={() => onChange(toggleNoTags(filters))}>
              {t('filter.noTags')}
            </Chip>
            {tagOptions.map((tag) => (
              <Chip
                key={tag.name}
                selected={filters.tags.some((name) => name.toLowerCase() === tag.name.toLowerCase())}
                onClick={() => onChange(toggleTag(filters, tag.name))}
              >
                {tagText(tag.name, t)}
                {tag.count > 0 && <span className="opacity-60">{tag.count}</span>}
              </Chip>
            ))}
            {tagOptions.length === 0 && <p className="text-xs text-muted-foreground">{t('filter.noTagOptions')}</p>}
          </ChipGroup>

          <ChipGroup title={t('filter.groups.columns')}>
            {columns.map((column) => (
              <Chip
                key={column.id}
                selected={filters.columns.includes(column.id)}
                onClick={() => onChange(toggleColumn(filters, column.id))}
              >
                {columnName(column.name)}
              </Chip>
            ))}
          </ChipGroup>

          <ChipGroup title={t('filter.groups.status')}>
            {STATE_FLAGS.map((state) => (
              <Chip key={state} selected={filters.states.includes(state)} onClick={() => onChange(toggleState(filters, state))}>
                {t(`filter.states.${state}`)}
              </Chip>
            ))}
          </ChipGroup>

          <ChipGroup title={t('filter.groups.priority')}>
            {PRIORITY_NAMES.map((priority) => (
              <Chip
                key={priority}
                selected={filters.priorities.includes(priority)}
                onClick={() => onChange(togglePriority(filters, priority))}
              >
                {t(`filter.priority.${priority}`)}
              </Chip>
            ))}
          </ChipGroup>

          {claimerOptions.length > 0 && (
            <ChipGroup title={t('filter.groups.claimedBy')}>
              {claimerOptions.map((name) => (
                <Chip key={name} selected={filters.claimers.includes(name)} onClick={() => onChange(toggleClaimer(filters, name))}>
                  {name}
                </Chip>
              ))}
            </ChipGroup>
          )}

          <ChipGroup title={t('filter.groups.due')} radioGroup>
            {DUE_CHOICES.map((due) => (
              <Chip key={due} radio selected={filters.due === due} onClick={() => onChange({ ...filters, due })}>
                {t(`filter.due.${due}`)}
              </Chip>
            ))}
          </ChipGroup>
        </div>
      </PopoverContent>
    </Popover>
  )
}
