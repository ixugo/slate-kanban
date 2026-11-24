// 测试用：在指定查询键上挂一个迟迟不返回的请求，用来检查被测代码有没有把它连带中止。

import type { QueryClient, QueryKey } from '@tanstack/react-query'

/** 在途请求从发出到返回的时长（毫秒），要比被测代码里的异步步骤慢 */
const INFLIGHT_DELAY_MS = 50

/** 一个挂在查询键上的在途请求 */
export interface InflightQuery {
  /** 请求是否收到过中止信号 */
  aborted: () => boolean
  /** 请求的结果；被中止时以取消错误结束 */
  result: Promise<{ ok: true }>
}

/** 在 key 上发起一个请求，INFLIGHT_DELAY_MS 毫秒后才返回；staleTime 设为 0，保证缓存里已有数据时也会真的发出 */
export function startInflightQuery(queryClient: QueryClient, key: QueryKey): InflightQuery {
  let aborted = false
  const result = queryClient.fetchQuery({
    queryKey: key,
    staleTime: 0,
    queryFn: ({ signal }) =>
      new Promise<{ ok: true }>((resolve) => {
        signal.addEventListener('abort', () => {
          aborted = true
        })
        setTimeout(() => resolve({ ok: true }), INFLIGHT_DELAY_MS)
      }),
  })
  // 被中止时 result 会拒绝；先吞掉未处理的拒绝，断言时再用 result 本身判断
  result.catch(() => undefined)
  return { aborted: () => aborted, result }
}
