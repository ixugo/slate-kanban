// 深浅主题：浅色、深色、跟随系统三选一，选择存在 localStorage。
// 首屏的主题由 public/theme-init.js 在渲染前设置（避免闪白），这里负责之后的切换和跟随系统变化。
// 两处的存储键和取值要保持一致。

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { THEME_KEY, ThemeContext, type ResolvedTheme, type ThemeChoice } from './theme-context'

const DARK_QUERY = '(prefers-color-scheme: dark)'

// 读取保存的选择；没有、不合法或存储不可用都当作"跟随系统"。
function readChoice(): ThemeChoice {
  try {
    const saved = localStorage.getItem(THEME_KEY)
    return saved === 'light' || saved === 'dark' ? saved : 'system'
  } catch {
    return 'system'
  }
}

// 系统当前是不是深色；浏览器不支持 matchMedia 时按浅色处理。
function systemPrefersDark(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia(DARK_QUERY).matches
}

/** 提供主题状态，并把生效的主题同步到 <html> 的 dark 类和 color-scheme */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemeChoice>(readChoice)
  const [systemDark, setSystemDark] = useState<boolean>(systemPrefersDark)

  // 跟随系统：系统在深浅之间切换时更新状态。
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return
    const media = window.matchMedia(DARK_QUERY)
    const onChange = (e: MediaQueryListEvent) => setSystemDark(e.matches)
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }, [])

  const resolved: ResolvedTheme = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme

  useEffect(() => {
    const root = document.documentElement
    root.classList.toggle('dark', resolved === 'dark')
    root.style.colorScheme = resolved
  }, [resolved])

  const setTheme = useCallback((next: ThemeChoice) => {
    try {
      localStorage.setItem(THEME_KEY, next)
    } catch {
      // 存储不可用：这次切换仍然生效，只是刷新后不会记住
    }
    setThemeState(next)
  }, [])

  const value = useMemo(() => ({ theme, resolved, setTheme }), [theme, resolved, setTheme])
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}
