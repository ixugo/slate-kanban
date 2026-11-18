// 应用的标志：三根高低不同的竖条，最右一根是琥珀色，对应看板的三列和"正在处理"的信号。
// 颜色取自主题变量，深浅主题下都清晰；和 public/favicon.svg 画的是同一个图形。

import { cn } from '@/lib/utils'

/** 画出标志，size 等尺寸类通过 className 传入，默认 1.5rem 见方 */
export function Logo({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" aria-hidden="true" className={cn('size-6 shrink-0', className)}>
      <rect width="32" height="32" className="fill-foreground" />
      <rect x="6" y="7" width="5" height="18" className="fill-background" />
      <rect x="13.5" y="7" width="5" height="11" className="fill-background" />
      <rect x="21" y="7" width="5" height="14" className="fill-signal" />
    </svg>
  )
}
