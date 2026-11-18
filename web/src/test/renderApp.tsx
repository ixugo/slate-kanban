// 页面测试的统一入口：把主题、查询缓存、路由、提示条按和正式应用一样的顺序套起来，
// 并可以指定起始地址。悬浮提示的 Provider 由用到它的页面自己提供，这里不再套。

import { QueryClientProvider, type QueryClient } from '@tanstack/react-query'
import { render, type RenderResult } from '@testing-library/react'
import type { ReactNode } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { Toaster } from '@/components/ui/sonner'
import { createQueryClient } from '@/lib/queryClient'
import { ThemeProvider } from '@/lib/theme'
import '@/i18n'

interface Options {
  /** 起始地址，默认 / */
  route?: string
  queryClient?: QueryClient
}

/** 渲染一棵带全部外层提供者的组件树，返回测试库的结果和用到的查询缓存 */
export function renderApp(ui: ReactNode, options: Options = {}): RenderResult & { queryClient: QueryClient } {
  const queryClient = options.queryClient ?? createQueryClient()
  // 测试里不让失败请求自动重试，否则错误页要等好几秒才出现
  queryClient.setDefaultOptions({ queries: { retry: false, staleTime: 0 }, mutations: { retry: false } })
  const result = render(
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[options.route ?? '/']}>{ui}</MemoryRouter>
        <Toaster />
      </QueryClientProvider>
    </ThemeProvider>,
  )
  return Object.assign(result, { queryClient })
}
