import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ThemeProvider } from './theme'
import { THEME_KEY, useTheme } from './theme-context'

// 可以手动触发"系统主题变化"的 matchMedia 替身。
function stubMatchMedia(initialDark: boolean) {
  let dark = initialDark
  const listeners = new Set<(e: MediaQueryListEvent) => void>()
  vi.stubGlobal(
    'matchMedia',
    vi.fn().mockImplementation(() => ({
      get matches() {
        return dark
      },
      addEventListener: (_: string, fn: (e: MediaQueryListEvent) => void) => listeners.add(fn),
      removeEventListener: (_: string, fn: (e: MediaQueryListEvent) => void) => listeners.delete(fn),
    })),
  )
  return (next: boolean) => {
    dark = next
    for (const fn of listeners) fn({ matches: next } as MediaQueryListEvent)
  }
}

function Probe() {
  const { theme, resolved, setTheme } = useTheme()
  return (
    <div>
      <span data-testid="state">{`${theme}/${resolved}`}</span>
      <button onClick={() => setTheme('dark')}>dark</button>
      <button onClick={() => setTheme('light')}>light</button>
      <button onClick={() => setTheme('system')}>system</button>
    </div>
  )
}

const state = () => screen.getByTestId('state').textContent

describe('ThemeProvider', () => {
  beforeEach(() => document.documentElement.classList.remove('dark'))
  afterEach(() => vi.unstubAllGlobals())

  it('没有保存的选择时跟随系统：系统是深色就用深色', () => {
    stubMatchMedia(true)
    render(<ThemeProvider><Probe /></ThemeProvider>)
    expect(state()).toBe('system/dark')
    expect(document.documentElement).toHaveClass('dark')
  })

  it('手动选择深色、浅色，写入 localStorage 并切换 dark 类和 color-scheme', async () => {
    stubMatchMedia(false)
    render(<ThemeProvider><Probe /></ThemeProvider>)
    await userEvent.click(screen.getByText('dark'))
    expect(state()).toBe('dark/dark')
    expect(localStorage.getItem(THEME_KEY)).toBe('dark')
    expect(document.documentElement).toHaveClass('dark')
    expect(document.documentElement.style.colorScheme).toBe('dark')
    await userEvent.click(screen.getByText('light'))
    expect(state()).toBe('light/light')
    expect(document.documentElement).not.toHaveClass('dark')
    expect(document.documentElement.style.colorScheme).toBe('light')
  })

  it('启动时读取保存的选择，手动选择优先于系统', () => {
    stubMatchMedia(true)
    localStorage.setItem(THEME_KEY, 'light')
    render(<ThemeProvider><Probe /></ThemeProvider>)
    expect(state()).toBe('light/light')
    expect(document.documentElement).not.toHaveClass('dark')
  })

  it('保存的值不合法时当作跟随系统', () => {
    stubMatchMedia(false)
    localStorage.setItem(THEME_KEY, 'purple')
    render(<ThemeProvider><Probe /></ThemeProvider>)
    expect(state()).toBe('system/light')
  })

  it('跟随系统时，系统主题变化会实时反映；手动选择后不再受系统影响', async () => {
    const setSystemDark = stubMatchMedia(false)
    render(<ThemeProvider><Probe /></ThemeProvider>)
    expect(state()).toBe('system/light')
    act(() => setSystemDark(true))
    expect(state()).toBe('system/dark')
    expect(document.documentElement).toHaveClass('dark')
    await userEvent.click(screen.getByText('light'))
    act(() => setSystemDark(false))
    act(() => setSystemDark(true))
    expect(state()).toBe('light/light')
  })

  it('localStorage 写入失败时这次切换仍然生效', async () => {
    stubMatchMedia(false)
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('quota', 'QuotaExceededError')
    })
    render(<ThemeProvider><Probe /></ThemeProvider>)
    await userEvent.click(screen.getByText('dark'))
    expect(state()).toBe('dark/dark')
    vi.restoreAllMocks()
  })

  it('浏览器没有 matchMedia 时按浅色处理，不报错', () => {
    vi.stubGlobal('matchMedia', undefined)
    render(<ThemeProvider><Probe /></ThemeProvider>)
    expect(state()).toBe('system/light')
  })

  it('不在 ThemeProvider 里使用会明确报错', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => render(<Probe />)).toThrow('ThemeProvider')
    spy.mockRestore()
  })
})
