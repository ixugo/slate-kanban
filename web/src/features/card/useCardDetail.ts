// 按主键加载卡片详情。

import { useQuery } from '@tanstack/react-query'
import { getCardDetail } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'

/** 读取单张卡片详情 */
export function useCardDetail(cardId: number) {
  return useQuery({
    queryKey: queryKeys.card(cardId),
    queryFn: ({ signal }) => getCardDetail(cardId, signal),
  })
}