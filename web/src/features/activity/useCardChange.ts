// 读一次任务变更记录（修改前、修改后的内容）。点「查看变更」打开对比弹窗时才会发请求。

import { useQuery } from '@tanstack/react-query'
import { getCardChange } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'

/** 变更记录只增不改，读到一次就不会再变，所以永不过期：同一条记录再次打开直接用缓存 */
export function useCardChange(cardId: number, changeId: number) {
  return useQuery({
    queryKey: queryKeys.cardChange(cardId, changeId),
    queryFn: ({ signal }) => getCardChange(cardId, changeId, signal),
    staleTime: Infinity,
  })
}
