import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/lib/api/client'
import { queryKeys } from '@/lib/queryKeys'
import { createQueryClient } from './queryClient'

afterEach(() => vi.restoreAllMocks())

// 让一个会失败的查询跑完，错误会走到全局的出错处理
async function failQuery(queryKey: readonly unknown[], error: unknown) {
  const client = createQueryClient()
  const invalidate = vi.spyOn(client, 'invalidateQueries')
  await client.fetchQuery({ queryKey, queryFn: () => Promise.reject(error), retry: false }).catch(() => undefined)
  return invalidate
}

describe('createQueryClient 的 401 处理', () => {
  it('查询返回 401：重新读取口令状态，并且不取消正在进行的那次', async () => {
    const invalidate = await failQuery(queryKeys.boards, new ApiError(401, 'ErrUnauthorized', '需要访问口令'))
    expect(invalidate).toHaveBeenCalledTimes(1)
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.auth }, { cancelRefetch: false })
  })

  it('变更（POST 等）返回 401 也一样', async () => {
    const client = createQueryClient()
    const invalidate = vi.spyOn(client, 'invalidateQueries')
    await client
      .getMutationCache()
      .build(client, { mutationFn: () => Promise.reject(new ApiError(401, 'ErrUnauthorized', '需要访问口令')) })
      .execute(undefined)
      .catch(() => undefined)
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.auth }, { cancelRefetch: false })
  })

  it('其他状态码的错误不触发', async () => {
    for (const status of [0, 400, 403, 404, 429, 500]) {
      const invalidate = await failQuery(queryKeys.boards, new ApiError(status, 'ErrOther', 'x'))
      expect(invalidate, `状态码 ${status}`).not.toHaveBeenCalled()
    }
  })

  it('不是接口错误（普通异常）不触发', async () => {
    const invalidate = await failQuery(queryKeys.boards, new Error('boom'))
    expect(invalidate).not.toHaveBeenCalled()
  })

  it('口令状态接口自己返回 401 时不触发，避免无限循环', async () => {
    const invalidate = await failQuery(queryKeys.auth, new ApiError(401, 'ErrUnauthorized', '网关拒绝'))
    expect(invalidate).not.toHaveBeenCalled()
  })
})
