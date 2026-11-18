// 活动流的查询：从最新的事件往回翻页。缓存键挂在看板键下面，
// 所以看板有更新（自己、别人或 agent 的操作）时，打开着的活动流会跟着重新读取。

import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query'
import { listActivity } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'
import { toQuery, type ActivityFilters } from './filters'

/** 一页读多少条；点「加载更早的活动」再读下一页 */
export const ACTIVITY_PAGE_SIZE = 30

/** 读某块看板的活动流；筛选条件一变就换一份缓存，新结果回来之前先显示上一份，列表不会闪空白 */
export function useActivity(boardId: number, filters: ActivityFilters) {
  const query = toQuery(filters)
  return useInfiniteQuery({
    queryKey: queryKeys.activity(boardId, filters),
    queryFn: ({ pageParam, signal }) =>
      listActivity(boardId, { ...query, before: pageParam, limit: ACTIVITY_PAGE_SIZE }, signal),
    // 第一页不带 before，之后每页用上一页最后一条（最旧的）的序号往前翻
    initialPageParam: undefined as number | undefined,
    getNextPageParam: (last) => (last.has_more ? last.events[last.events.length - 1]?.id : undefined),
    placeholderData: keepPreviousData,
  })
}
