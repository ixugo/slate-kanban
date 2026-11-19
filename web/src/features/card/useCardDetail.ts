// 按地址栏 ?card= 编号加载卡片详情。

import { useQuery } from '@tanstack/react-query'
import { getCardDetail } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'
import { parseId } from '@/lib/route'

/** 读取单张卡片详情；cardParam 来自 searchParams */
export function useCardDetail(cardParam: string | null) {
  const cardId = parseId(cardParam)
  return useQuery({
    queryKey: queryKeys.card(cardId ?? 0),
    queryFn: ({ signal }) => getCardDetail(cardId as number, signal),
    enabled: cardId !== null,
  })
}