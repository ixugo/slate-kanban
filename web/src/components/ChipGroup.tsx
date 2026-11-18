// 药丸形的可点选小标签（Chip）和带小标题的一组小标签（ChipGroup）：看板的筛选弹层和活动流抽屉共用。

import { useId, type ReactNode } from 'react'
import { cn } from '@/lib/utils'

/** 选项小标签的公共样式：药丸形，选中时填充主色 */
const CHIP =
  'inline-flex h-7 items-center gap-1.5 rounded-full px-3 text-[0.8125rem] font-medium transition-colors duration-200 ease-spring outline-none select-none focus-visible:ring-[3px] focus-visible:ring-ring/40 active:scale-[0.97]'
const CHIP_ON = 'bg-primary text-primary-foreground'
const CHIP_OFF = 'bg-secondary text-foreground/80 hover:bg-accent'

interface ChipProps {
  selected: boolean
  onClick: () => void
  /** 单选组里的选项用 radio 语义，多选用 pressed 语义 */
  radio?: boolean
  children: ReactNode
}

/** 一个可点选的小标签 */
export function Chip({ selected, onClick, radio, children }: ChipProps) {
  const state = radio ? { role: 'radio' as const, 'aria-checked': selected } : { 'aria-pressed': selected }
  return (
    <button type="button" {...state} onClick={onClick} className={cn(CHIP, selected ? CHIP_ON : CHIP_OFF)}>
      {children}
    </button>
  )
}

interface ChipGroupProps {
  title: string
  /** 标题右侧的附加控件 */
  aside?: ReactNode
  /** 单选组要标成 radiogroup */
  radioGroup?: boolean
  children: ReactNode
}

/** 一组小标签：小标题加一排小标签 */
export function ChipGroup({ title, aside, radioGroup, children }: ChipGroupProps) {
  const titleId = useId()
  return (
    <div className="flex flex-col gap-2">
      <div className="flex min-h-5 items-center justify-between gap-2">
        <h3 id={titleId} className="text-xs font-medium text-muted-foreground">
          {title}
        </h3>
        {aside}
      </div>
      <div
        role={radioGroup ? 'radiogroup' : 'group'}
        aria-labelledby={titleId}
        className="flex flex-wrap gap-1.5"
      >
        {children}
      </div>
    </div>
  )
}
