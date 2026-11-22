import { QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { updateCard } from '@/lib/api/endpoints'
import type { CardDetail, Snapshot } from '@/lib/api/types'
import { createQueryClient } from '@/lib/queryClient'
import { queryKeys } from '@/lib/queryKeys'
import { makeCard, makeSnapshot } from '@/test/factories'
import { useCardUpdate } from './useCardUpdate'

vi.mock('@/lib/api/endpoints', () => ({ updateCard: vi.fn() }))
const toastError = vi.fn()
vi.mock('@/lib/useErrorToast', () => ({ useErrorToast: () => toastError }))

const updateApi = vi.mocked(updateCard)

const detail = { ...makeCard({ id: 1, title: '旧标题', priority: 1 }), column_instruction: '', dependencies: [], comments: [] } as CardDetail

function setup() {
  const queryClient = createQueryClient()
  queryClient.setQueryData(queryKeys.card(1), detail)
  queryClient.setQueryData(queryKeys.board(1), makeSnapshot({ cards: [makeCard({ id: 1, title: '旧标题', priority: 1 })] }))
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  const hook = renderHook(() => useCardUpdate(), { wrapper })
  const title = () => queryClient.getQueryData<CardDetail>(queryKeys.card(1))?.title
  const tileTitle = () => queryClient.getQueryData<Snapshot>(queryKeys.board(1))?.cards[0].title
  return { ...hook, title, tileTitle }
}

describe('useCardUpdate 乐观更新', () => {
  afterEach(() => vi.clearAllMocks())

  it('请求还没返回时，详情和看板磁贴已经是改后的样子', async () => {
    updateApi.mockReturnValue(new Promise(() => {}))
    const { result, title, tileTitle } = setup()
    act(() => result.current.mutate({ cardId: 1, boardId: 1, input: { title: '新标题', version: 1 } }))
    await waitFor(() => expect(title()).toBe('新标题'))
    expect(tileTitle()).toBe('新标题')
  })

  it('失败后两处都还原并提示错误', async () => {
    updateApi.mockRejectedValue(new Error('boom'))
    const { result, title, tileTitle } = setup()
    await act(async () => result.current.mutate({ cardId: 1, boardId: 1, input: { title: '新标题', version: 1 } }))
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(title()).toBe('旧标题')
    expect(tileTitle()).toBe('旧标题')
    expect(toastError).toHaveBeenCalled()
  })
})
