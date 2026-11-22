// 移动任务：先在本地缓存里预测结果（乐观更新），失败就恢复，成功后以服务端的数据为准。

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { moveCard } from '@/lib/api/endpoints'
import type { Snapshot } from '@/lib/api/types'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'
import { applyMoveToSnapshot, type MovePlan } from './dnd'

/** 一次移动的参数 */
export interface MoveVariables {
  cardId: number
  plan: MovePlan
}

/** 返回移动任务的变更；拖拽和任务弹窗里换列共用它，规则和提示一致 */
export function useMoveCard(boardId: number) {
  const queryClient = useQueryClient()
  const toastError = useErrorToast()
  const key = queryKeys.board(boardId)

  return useMutation({
    mutationFn: ({ cardId, plan }: MoveVariables) =>
      moveCard(cardId, { column_id: plan.columnId, before_id: plan.beforeId, after_id: plan.afterId }),
    // 先取消正在飞的快照请求，免得它回来把乐观结果盖掉；再记下旧快照，失败时恢复
    onMutate: async ({ cardId, plan }) => {
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<Snapshot>(key)
      if (previous) queryClient.setQueryData(key, applyMoveToSnapshot(previous, cardId, plan))
      return { previous }
    },
    onError: (err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(key, context.previous)
      toastError(err)
    },
    // 无论成败都重新取一次：成功时校准位置和自动化规则的副作用，失败时确认回到服务端的真实状态
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: key })
      void queryClient.invalidateQueries({ queryKey: queryKeys.card() })
    },
  })
}
