// 设置页的变更：看板、列、规则；成功后刷新快照，改名称时顺带刷新看板列表。

import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  createColumn,
  createRule,
  deleteBoard,
  deleteColumn,
  deleteRule,
  reorderColumns,
  updateBoard,
  updateColumn,
  updateRule,
} from '@/lib/api/endpoints'
import type {
  CreateColumnInput,
  CreateRuleInput,
  Snapshot,
  UpdateColumnInput,
  UpdateRuleInput,
} from '@/lib/api/types'
import { queryKeys } from '@/lib/queryKeys'

/** 绑定到一块看板的全部变更钩子 */
export function useSettingsMutations(boardId: number) {
  const queryClient = useQueryClient()
  const boardKey = queryKeys.board(boardId)

  const invalidateBoard = () => {
    void queryClient.invalidateQueries({ queryKey: boardKey })
  }

  const invalidateBoardAndList = () => {
    invalidateBoard()
    void queryClient.invalidateQueries({ queryKey: queryKeys.boards })
  }

  const patchSnapshot = (patch: (s: Snapshot) => Snapshot) => {
    const prev = queryClient.getQueryData<Snapshot>(boardKey)
    if (prev) queryClient.setQueryData(boardKey, patch(prev))
    return prev
  }

  const updateBoardM = useMutation({
    mutationFn: (input: Parameters<typeof updateBoard>[1]) => updateBoard(boardId, input),
    onSuccess: () => invalidateBoardAndList(),
  })

  const deleteBoardM = useMutation({
    mutationFn: () => deleteBoard(boardId),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: boardKey })
      void queryClient.invalidateQueries({ queryKey: queryKeys.boards })
    },
  })

  const createColumnM = useMutation({
    mutationFn: (input: CreateColumnInput) => createColumn(boardId, input),
    onSuccess: invalidateBoard,
  })

  const updateColumnM = useMutation({
    mutationFn: ({ columnId, input }: { columnId: number; input: UpdateColumnInput }) =>
      updateColumn(boardId, columnId, input),
    onSuccess: invalidateBoard,
  })

  const reorderColumnsM = useMutation({
    mutationFn: (ids: number[]) => reorderColumns(boardId, ids),
    onMutate: async (ids) => {
      await queryClient.cancelQueries({ queryKey: boardKey })
      const previous = patchSnapshot((s) => ({
        ...s,
        columns: ids
          .map((id, index) => {
            const col = s.columns.find((c) => c.id === id)
            return col ? { ...col, sort: (index + 1) * 1024 } : undefined
          })
          .filter((c): c is Snapshot['columns'][number] => c !== undefined),
      }))
      return { previous }
    },
    onError: (_e, _v, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(boardKey, ctx.previous)
    },
    onSettled: invalidateBoard,
  })

  const deleteColumnM = useMutation({
    mutationFn: ({ columnId, moveTo }: { columnId: number; moveTo?: number }) =>
      deleteColumn(boardId, columnId, moveTo),
    onSuccess: invalidateBoard,
  })

  const createRuleM = useMutation({
    mutationFn: (input: CreateRuleInput) => createRule(boardId, input),
    onSuccess: invalidateBoard,
  })

  const updateRuleM = useMutation({
    mutationFn: ({ ruleId, input }: { ruleId: number; input: UpdateRuleInput }) => updateRule(ruleId, input),
    onMutate: async ({ ruleId, input }) => {
      await queryClient.cancelQueries({ queryKey: boardKey })
      const previous = patchSnapshot((s) => ({
        ...s,
        rules: s.rules.map((r) =>
          r.id === ruleId
            ? {
                ...r,
                ...(input.name !== undefined ? { name: input.name } : {}),
                ...(input.enabled !== undefined ? { enabled: input.enabled } : {}),
                ...(input.actions !== undefined ? { actions: input.actions } : {}),
              }
            : r,
        ),
      }))
      return { previous }
    },
    onError: (_e, _v, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(boardKey, ctx.previous)
    },
    onSettled: invalidateBoard,
  })

  const deleteRuleM = useMutation({
    mutationFn: (ruleId: number) => deleteRule(ruleId),
    onSuccess: invalidateBoard,
  })

  const toggleRuleM = useMutation({
    mutationFn: ({ ruleId, enabled }: { ruleId: number; enabled: boolean }) => updateRule(ruleId, { enabled }),
    onMutate: async ({ ruleId, enabled }) => {
      await queryClient.cancelQueries({ queryKey: boardKey })
      const previous = patchSnapshot((s) => ({
        ...s,
        rules: s.rules.map((r) => (r.id === ruleId ? { ...r, enabled } : r)),
      }))
      return { previous }
    },
    onError: (_e, _v, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(boardKey, ctx.previous)
    },
    onSettled: invalidateBoard,
  })

  return {
    updateBoard: updateBoardM,
    deleteBoard: deleteBoardM,
    createColumn: createColumnM,
    updateColumn: updateColumnM,
    reorderColumns: reorderColumnsM,
    deleteColumn: deleteColumnM,
    createRule: createRuleM,
    updateRule: updateRuleM,
    deleteRule: deleteRuleM,
    toggleRule: toggleRuleM,
  }
}