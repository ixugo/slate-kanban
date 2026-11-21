/** 打开任务详情时把未读数乐观清零，失败则恢复本地快照 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { markCardRead } from '@/lib/api/endpoints'
import type { Snapshot } from '@/lib/api/types'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'

/** 在 boardId 快照里把指定任务的 unread_count 置 0 */
function zeroUnreadInSnapshot(snapshot: Snapshot, cardId: number): Snapshot {
  return {
    ...snapshot,
    cards: snapshot.cards.map((c) => (c.id === cardId ? { ...c, unread_count: 0 } : c)),
  }
}

/** 若快照里该任务有未读，则调用已读接口并乐观更新缓存 */
export function useMarkCardRead(boardId: number) {
  const queryClient = useQueryClient()
  const toastError = useErrorToast()
  const key = queryKeys.board(boardId)

  const mutation = useMutation({
    mutationFn: (cardId: number) => markCardRead(cardId),
    onMutate: async (cardId) => {
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<Snapshot>(key)
      if (previous) queryClient.setQueryData(key, zeroUnreadInSnapshot(previous, cardId))
      return { previous }
    },
    onError: (err, _cardId, context) => {
      if (context?.previous) queryClient.setQueryData(key, context.previous)
      toastError(err)
    },
  })

  /** 仅当本地快照显示有未读时才发起请求 */
  function tryMark(cardId: number) {
    const snap = queryClient.getQueryData<Snapshot>(key)
    const card = snap?.cards.find((c) => c.id === cardId)
    if (!card || (card.unread_count ?? 0) <= 0) return
    mutation.mutate(cardId)
  }

  return { tryMark }
}