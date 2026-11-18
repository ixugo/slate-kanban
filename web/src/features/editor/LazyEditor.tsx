// 编辑器的懒加载入口：编辑器连同它的解析、命令和样式是单独的一块，
// 只有用户真的要写字时才下载，首屏体积不受影响。

import { lazy, Suspense, type ComponentProps } from 'react'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

const LiveEditor = lazy(() => import('./LiveEditor'))

/** 和 LiveEditor 用法完全一样；加载期间显示一块和编辑区等高的占位 */
export function LazyEditor(props: ComponentProps<typeof LiveEditor>) {
  return (
    <Suspense fallback={<Skeleton aria-busy="true" className={cn('w-full rounded-xl', props.compact ? 'h-28' : 'h-44')} />}>
      <LiveEditor {...props} />
    </Suspense>
  )
}
