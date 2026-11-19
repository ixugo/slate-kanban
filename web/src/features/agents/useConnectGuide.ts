// 接入说明的查询。

import { useQuery } from '@tanstack/react-query'
import { getConnectGuide } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'

/** 读取 GET /connect 的接入说明 */
export function useConnectGuide() {
  return useQuery({
    queryKey: queryKeys.connect,
    queryFn: ({ signal }) => getConnectGuide(signal),
  })
}