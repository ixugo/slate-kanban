import { describe, expect, it } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { createQueryClient } from '@/lib/queryClient'
import { queryKeys } from '@/lib/queryKeys'
import { makeSnapshot, makeCard } from '@/test/factories'
import { installFakeApi } from '@/test/fakeApi'
import { useMarkCardRead } from './useMarkCardRead'

function wrapper(client: ReturnType<typeof createQueryClient>) {
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
}

describe('useMarkCardRead', () => {
  it('乐观清零并调用已读接口', async () => {
    const api = installFakeApi({
      'POST /api/v1/cards/:id/read': {},
    })
    const client = createQueryClient()
    const snap = makeSnapshot({
      cards: [makeCard({ id: 9, unread_count: 2 })],
    })
    client.setQueryData(queryKeys.board(1), snap)
    const { result } = renderHook(() => useMarkCardRead(1), { wrapper: wrapper(client) })
    result.current.tryMark(9)
    await waitFor(() => expect(api.calls.some((c) => c.path.endsWith('/cards/9/read'))).toBe(true))
    const cached = client.getQueryData<typeof snap>(queryKeys.board(1))
    expect(cached?.cards[0].unread_count).toBe(0)
  })
})