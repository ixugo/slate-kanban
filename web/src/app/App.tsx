// 应用的根组件：把主题、查询缓存、路由、提示气泡按顺序套起来。

import { QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { createQueryClient } from '@/lib/queryClient'
import { ThemeProvider } from '@/lib/theme'
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
          <TooltipProvider delayDuration={300}>
            <BrowserRouter>
              <AppRoutes />
            </BrowserRouter>
            <Toaster closeButton />
          </TooltipProvider>
        </QueryClientProvider>
      </ThemeProvider>
    </ErrorBoundary>
  )
}
