// 主题的上下文、类型和读取它的钩子。和 ThemeProvider 分开放，是为了让 theme.tsx 只导出组件（热更新才能正常工作）。

import { createContext, useContext } from 'react'

/** 用户的选择 */
export type ThemeChoice = 'light' | 'dark' | 'system'
/** 实际生效的主题 */
export type ResolvedTheme = 'light' | 'dark'

/** 主题选择存在 localStorage 的这个键里；public/theme-init.js 用同一个键 */
export const THEME_KEY = 'slate.theme'

export interface ThemeContextValue {
  theme: ThemeChoice
  resolved: ResolvedTheme
  setTheme: (theme: ThemeChoice) => void
}

export const ThemeContext = createContext<ThemeContextValue | null>(null)

/** 读取和切换主题；必须在 ThemeProvider 里使用 */
export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme 必须在 ThemeProvider 里使用')
  return ctx
}
