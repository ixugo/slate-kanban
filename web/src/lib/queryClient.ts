// 全局唯一的查询缓存和它的默认行为。

import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'
import { ApiError } from '@/lib/api/client'
import { queryKeys } from '@/lib/queryKeys'

/** 网络错误和服务端错误最多自动重试的次数 */
const MAX_RETRIES = 2

// 只有"也许下次就好了"的失败才重试：网络不通和 5xx；4xx 是请求本身有问题，重试没有意义。
function shouldRetry(failureCount: number, error: unknown): boolean {
  if (failureCount >= MAX_RETRIES) return false
  if (error instanceof ApiError) return error.status === 0 || error.status >= 500
  return false
}

// 任何接口返回 401：可能是登录过期，也可能服务端刚启用了访问口令。
// 不在这里直接假定要登录，而是重新问一次服务端的口令状态：它说需要登录，页面才切到登录页；
// 同时进行的多个请求一起 401 时，只发一次状态请求，不互相取消重发。
// 口令状态接口自己返回 401（比如前面有别的网关拦截了所有请求）时不能再重读，否则会无限循环。
function handleUnauthorized(client: QueryClient, error: unknown, queryKey?: readonly unknown[]): void {
  if (!(error instanceof ApiError) || error.status !== 401) return
  if (queryKey?.[0] === queryKeys.auth[0]) return
  void client.invalidateQueries({ queryKey: queryKeys.auth }, { cancelRefetch: false })
}

/** 创建查询缓存；测试里每个用例单独创建一个，互不影响 */
export function createQueryClient(): QueryClient {
  const client: QueryClient = new QueryClient({
    queryCache: new QueryCache({ onError: (error, query) => handleUnauthorized(client, error, query.queryKey) }),
    mutationCache: new MutationCache({ onError: (error) => handleUnauthorized(client, error) }),
    defaultOptions: {
      queries: { retry: shouldRetry, refetchOnWindowFocus: true, staleTime: 5_000 },
      mutations: { retry: false },
    },
  })
  return client
}
