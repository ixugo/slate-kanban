// 全局唯一的查询缓存和它的默认行为。

import { QueryClient } from '@tanstack/react-query'
import { ApiError } from '@/lib/api/client'

/** 网络错误和服务端错误最多自动重试的次数 */
const MAX_RETRIES = 2

// 只有"也许下次就好了"的失败才重试：网络不通和 5xx；4xx 是请求本身有问题，重试没有意义。
function shouldRetry(failureCount: number, error: unknown): boolean {
  if (failureCount >= MAX_RETRIES) return false
  if (error instanceof ApiError) return error.status === 0 || error.status >= 500
  return false
}

/** 创建查询缓存；测试里每个用例单独创建一个，互不影响 */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: shouldRetry, refetchOnWindowFocus: true, staleTime: 5_000 },
      mutations: { retry: false },
    },
  })
}
