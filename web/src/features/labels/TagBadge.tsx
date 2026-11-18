// 卡片上的一枚标签：库里配了颜色就用该颜色，否则是默认的灰色小标签。

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { useLabelStyle } from './useLabels'

interface TagBadgeProps {
  name: string
  className?: string
}

/** 按名称显示一枚标签 */
export function TagBadge({ name, className }: TagBadgeProps) {
  const style = useLabelStyle(name)
  return (
    <Badge variant="secondary" style={style} className={cn('max-w-[8rem] truncate', className)}>
      {name}
    </Badge>
  )
}
