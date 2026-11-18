// 登录状态的上下文和读取它的钩子。和 AuthProvider 分开放，是为了让 auth.tsx 只导出组件（热更新才能正常工作）。

import { createContext, useContext } from 'react'

/** 令牌有效，但不是管理令牌（属于 agent）时使用的 reason */
export const REASON_NOT_HUMAN = 'ErrNotHuman'

export interface AuthContextValue {
  /** 当前保存的令牌，空串表示没登录 */
  token: string
  /** 校验并保存令牌；失败时抛出 ApiError（401 令牌无效，ErrNotHuman 不是管理令牌） */
  login: (token: string) => Promise<void>
  /** 退出：清掉令牌 */
  logout: () => void
}

export const AuthContext = createContext<AuthContextValue | null>(null)

/** 读取登录状态；必须在 AuthProvider 里使用 */
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth 必须在 AuthProvider 里使用')
  return ctx
}
