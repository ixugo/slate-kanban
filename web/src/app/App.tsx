// 应用的根组件：把主题、查询缓存、路由、提示条按顺序套起来。
// 悬浮提示（Tooltip）的 Provider 不放在这里：它依赖的 Radix 与定位库较重，由用到它的页面自己提供，首屏不用下载。

import { QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import { Toaster } from '@/components/ui/sonner'
import { createQueryClient } from '@/lib/queryClient'
import { ThemeProvider } from '@/lib/theme'
import { OfflineBanner } from '@/pwa/OfflineBanner'
import { AppRoutes } from './AppRoutes'
import { ErrorBoundary } from './ErrorBoundary'

// 全局唯一的查询缓存，在整个页面的生命周期里一直使用
const queryClient = createQueryClient()

/** 根组件 */
export function App() {
  return (
    <ErrorBoundary>
      <ThemeProvider>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <OfflineBanner />
            <AppRoutes />
          </BrowserRouter>
          <Toaster closeButton />
        </QueryClientProvider>
      </ThemeProvider>
    </ErrorBoundary>
  )
}
