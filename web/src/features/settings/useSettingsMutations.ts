// 设置页的变更：看板、列、规则。改名、改列、改规则、删规则先改快照再请求，失败还原并提示；成功后刷新快照，改名称时顺带刷新看板列表。

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
import { useErrorToast } from '@/lib/useErrorToast'

/** 绑定到一块看板的全部变更钩子 */
export function useSettingsMutations(boardId: number) {
  const queryClient = useQueryClient()
  const toastError = useErrorToast()
  const boardKey = queryKeys.board(boardId)

  const invalidateBoard = () => {
    void queryClient.invalidateQueries({ queryKey: boardKey })
  }

  const invalidateBoardAndList = () => {
    invalidateBoard()
    void queryClient.invalidateQueries({ queryKey: queryKeys.boards })
  }

  // 只取消整板快照的在途请求，免得它回来盖掉乐观结果；
  // 任务详情的活动流等查询也挂在看板键下面，设置页改动时它们可能正在加载，不能被连带中止
  const cancelSnapshot = () => queryClient.cancelQueries({ queryKey: boardKey, exact: true })

  const patchSnapshot = (patch: (s: Snapshot) => Snapshot) => {
    const prev = queryClient.getQueryData<Snapshot>(boardKey)
    if (prev) queryClient.setQueryData(boardKey, patch(prev))
    return prev
  }

  const updateBoardM = useMutation({
    mutationFn: (input: Parameters<typeof updateBoard>[1]) => updateBoard(boardId, input),
    // 名称和说明先改快照；工程师和标签的新增要服务端分配编号，不预测
    onMutate: async (input) => {
      await cancelSnapshot()
      const previous = patchSnapshot((s) => ({
        ...s,
        ...(input.name !== undefined ? { name: input.name } : {}),
        ...(input.description !== undefined ? { description: input.description } : {}),
      }))
      return { previous }
    },
    onError: (err, _v, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(boardKey, ctx.previous)
      toastError(err)
    },
    // 工作目录必须由服务端确认后才写入快照，避免回显已被拒绝的路径。
    onSuccess: (saved, input) => {
      if (input.project_path !== undefined) patchSnapshot((s) => ({ ...s, project_path: saved.project_path }))
    },
    onSettled: () => invalidateBoardAndList(),
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
    onMutate: async ({ columnId, input }) => {
      await cancelSnapshot()
      const previous = patchSnapshot((s) => ({
        ...s,
        columns: s.columns.map((c) => (c.id === columnId ? { ...c, ...input } : c)),
      }))
      return { previous }
    },
    onError: (err, _v, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(boardKey, ctx.previous)
      toastError(err)
    },
    onSettled: invalidateBoard,
  })

  const reorderColumnsM = useMutation({
    mutationFn: (ids: number[]) => reorderColumns(boardId, ids),
    onMutate: async (ids) => {
      await cancelSnapshot()
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
    onError: (err, _v, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(boardKey, ctx.previous)
      toastError(err)
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
      await cancelSnapshot()
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
    onError: (err, _v, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(boardKey, ctx.previous)
      toastError(err)
    },
    onSettled: invalidateBoard,
  })

  const deleteRuleM = useMutation({
    mutationFn: (ruleId: number) => deleteRule(ruleId),
    onMutate: async (ruleId) => {
      await cancelSnapshot()
      const previous = patchSnapshot((s) => ({ ...s, rules: s.rules.filter((r) => r.id !== ruleId) }))
      return { previous }
    },
    onError: (err, _v, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(boardKey, ctx.previous)
      toastError(err)
    },
    onSettled: invalidateBoard,
  })

  const toggleRuleM = useMutation({
    mutationFn: ({ ruleId, enabled }: { ruleId: number; enabled: boolean }) => updateRule(ruleId, { enabled }),
    onMutate: async ({ ruleId, enabled }) => {
      await cancelSnapshot()
      const previous = patchSnapshot((s) => ({
        ...s,
        rules: s.rules.map((r) => (r.id === ruleId ? { ...r, enabled } : r)),
      }))
      return { previous }
    },
    onError: (err, _v, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(boardKey, ctx.previous)
      toastError(err)
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
