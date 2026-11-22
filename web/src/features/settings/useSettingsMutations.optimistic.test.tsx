import { QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { deleteRule, updateBoard, updateColumn } from '@/lib/api/endpoints'
import type { Rule, Snapshot } from '@/lib/api/types'
import { createQueryClient } from '@/lib/queryClient'
import { queryKeys } from '@/lib/queryKeys'
import { makeSnapshot } from '@/test/factories'
import { useSettingsMutations } from './useSettingsMutations'

vi.mock('@/lib/api/endpoints', () => ({ deleteRule: vi.fn(), updateBoard: vi.fn(), updateColumn: vi.fn() }))
const toastError = vi.fn()
vi.mock('@/lib/useErrorToast', () => ({ useErrorToast: () => toastError }))

const rule = { id: 3, board_id: 1, name: '规则', enabled: true } as Rule

function setup() {
  const queryClient = createQueryClient()
  queryClient.setQueryData(queryKeys.board(1), makeSnapshot({ rules: [rule] }))
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  const hook = renderHook(() => useSettingsMutations(1), { wrapper })
  const snap = () => queryClient.getQueryData<Snapshot>(queryKeys.board(1)) as Snapshot
  return { ...hook, snap }
}

describe('设置页乐观更新', () => {
  afterEach(() => vi.clearAllMocks())

  it('删除规则：请求未返回规则已消失，失败后回来并提示', async () => {
    vi.mocked(deleteRule).mockReturnValue(new Promise(() => {}))
    const first = setup()
    act(() => first.result.current.deleteRule.mutate(3))
    await waitFor(() => expect(first.snap().rules).toEqual([]))

    vi.mocked(deleteRule).mockRejectedValue(new Error('boom'))
    const second = setup()
    await act(async () => second.result.current.deleteRule.mutate(3))
    await waitFor(() => expect(second.result.current.deleteRule.isError).toBe(true))
    expect(second.snap().rules).toHaveLength(1)
    expect(toastError).toHaveBeenCalled()
  })

  it('改列：名称先变，失败后还原', async () => {
    vi.mocked(updateColumn).mockRejectedValue(new Error('boom'))
    const { result, snap } = setup()
    await act(async () => result.current.updateColumn.mutate({ columnId: 1, input: { name: '新名' } }))
    await waitFor(() => expect(result.current.updateColumn.isError).toBe(true))
    expect(snap().columns.find((c) => c.id === 1)?.name).toBe('待办')
  })

  it('改项目名称：先变，失败后还原', async () => {
    vi.mocked(updateBoard).mockReturnValue(new Promise(() => {}))
    const { result, snap } = setup()
    act(() => result.current.updateBoard.mutate({ name: '新项目名' }))
    await waitFor(() => expect(snap().name).toBe('新项目名'))
  })
})
