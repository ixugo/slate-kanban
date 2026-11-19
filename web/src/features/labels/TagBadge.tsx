// 卡片上的一枚标签：有文字时是带颜色的小标签，没有文字时只是一条颜色条；没配颜色就用默认的灰色。

import { Badge } from '@/components/ui/badge'
import { useTranslation } from '@/i18n/react'
import { labelStyle } from '@/lib/labelColors'
import { cn } from '@/lib/utils'
import { tagName, tagText } from './tagName'
import { useTag } from './useLabels'

interface TagBadgeProps {
  /** 标签主键 */
  id: number
  /** 大号用在卡片弹窗的标签区；默认是磁贴和列表里的小号 */
  size?: 'sm' | 'lg'
  className?: string
}

/** 按主键显示一枚标签；库里查不到时什么都不画 */
export function TagBadge({ id, size = 'sm', className }: TagBadgeProps) {
  const { t } = useTranslation()
  const tag = useTag(id)
  if (!tag) return null
  const style = labelStyle(tag.color)
  if (!tag.text) {
    return <span role="img" aria-label={tagName(tag, t)} style={style} className={cn(size === 'lg' ? 'inline-block h-8 w-14 rounded-md' : 'inline-block h-2 w-9 rounded-full', !style && 'bg-muted-foreground/40', className)} />
  }
  return (
    <Badge variant="secondary" style={style} className={cn('max-w-[8rem] truncate', size === 'lg' && 'h-8 max-w-[14rem] rounded-md px-3 text-sm font-semibold', className)}>
      {tagText(tag.text, t)}
    </Badge>
  )
}
