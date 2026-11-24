/** 打开任务详情时把未读数乐观清零，失败则恢复本地快照 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useRef } from 'react'
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
  // 已发出已读请求、结果还没回来（或失败后）的任务编号。详情弹窗每次重新渲染都会再检查一遍未读，
  // 而乐观清零是异步的，请求落地前快照仍显示未读；不记下来，同一张任务会连发好几条一样的请求。
  // 请求成功后移除，详情开着又来了新评论时才能再标记；失败后保留，免得恢复的快照又触发无限重试，重新打开详情才会再试
  const requested = useRef(new Set<number>())

  const mutation = useMutation({
    mutationFn: (cardId: number) => markCardRead(cardId),
    onMutate: async (cardId) => {
      // 只中止整板快照：任务详情的活动流等查询也挂在看板键下面，刚打开详情时它们正在加载，不能被连带中止
      await queryClient.cancelQueries({ queryKey: key, exact: true })
      const previous = queryClient.getQueryData<Snapshot>(key)
      if (previous) queryClient.setQueryData(key, zeroUnreadInSnapshot(previous, cardId))
      return { previous }
    },
    onError: (err, _cardId, context) => {
      if (context?.previous) queryClient.setQueryData(key, context.previous)
      toastError(err)
    },
    onSuccess: (_data, cardId) => {
      requested.current.delete(cardId)
    },
  })

  /** 仅当本地快照显示有未读、且这张任务没有已读请求在途时才发起请求 */
  function tryMark(cardId: number) {
    if (requested.current.has(cardId)) return
    const snap = queryClient.getQueryData<Snapshot>(key)
    const card = snap?.cards.find((c) => c.id === cardId)
    if (!card || (card.unread_count ?? 0) <= 0) return
    requested.current.add(cardId)
    mutation.mutate(cardId)
  }

  return { tryMark }
}
