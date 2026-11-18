// 登录状态：令牌存在 localStorage（见 token.ts），这里负责登录校验、退出，以及身份变化时清空缓存。
// 网页只允许"人"登录：agent 令牌虽然能通过校验，但属于 MCP 和脚本，拿来登录网页会让所有管理操作报 403。

import { useCallback, useEffect, useMemo, useSyncExternalStore, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { getMe } from '@/lib/api/endpoints'
import { ApiError } from '@/lib/api/client'
import { clearToken, getToken, setToken, subscribeToken } from '@/lib/token'
import { AuthContext, REASON_NOT_HUMAN } from './auth-context'

/** 提供登录状态；令牌被清空（退出、接口返回 401）时同时清空所有缓存，不让上一个身份的数据残留 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const token = useSyncExternalStore(subscribeToken, getToken)
  const queryClient = useQueryClient()

  const login = useCallback(
    async (raw: string) => {
      const candidate = raw.trim()
      const me = await getMe(candidate)
      if (me.kind !== 'human') throw new ApiError(403, REASON_NOT_HUMAN, 'the web UI needs the admin token')
      queryClient.clear()
      setToken(candidate)
    },
    [queryClient],
  )

  const logout = useCallback(() => clearToken(), [])

  useEffect(() => {
    if (token === '') queryClient.clear()
  }, [token, queryClient])

  const value = useMemo(() => ({ token, login, logout }), [token, login, logout])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
