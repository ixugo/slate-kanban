// 读一张卡片的事件（建卡、修改、换列、认领、自动化），卡片详情的「评论和活动」用。
// 缓存键挂在看板键下面，所以别人或 agent 操作了这张卡片，实时刷新会让它自动重新读取。

import { useQuery } from '@tanstack/react-query'
import { listActivity } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'
import { CARD_EVENT_TYPES } from './timeline'

/** 最多读最近多少条；更早的事件在看板的「活动」抽屉里还能翻到 */
const CARD_EVENTS_LIMIT = 100

/** enabled 为假时不发请求（用户收起了详情） */
export function useCardActivity(boardId: number, cardId: number, enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.cardActivity(boardId, cardId),
    queryFn: ({ signal }) => listActivity(boardId, { card_id: cardId, types: CARD_EVENT_TYPES, limit: CARD_EVENTS_LIMIT }, signal),
    enabled,
  })
}
