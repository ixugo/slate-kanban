// 主题切换：单击在浅色 → 深色 → 跟随系统 之间循环，不用下拉菜单以减轻首屏体积。

import { MonitorIcon, MoonIcon, SunIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { useTheme, type ThemeChoice } from '@/lib/theme-context'

/** 循环顺序：浅色 → 深色 → 跟随系统 */
const THEME_CYCLE: ThemeChoice[] = ['light', 'dark', 'system']

const THEME_LABEL_KEY = {
  light: 'theme.light',
  dark: 'theme.dark',
  system: 'theme.system',
} as const satisfies Record<ThemeChoice, 'theme.light' | 'theme.dark' | 'theme.system'>

/** 取下一个主题选项 */
function nextThemeChoice(current: ThemeChoice): ThemeChoice {
  const index = THEME_CYCLE.indexOf(current)
  return THEME_CYCLE[(index + 1) % THEME_CYCLE.length]
}

/** 顶栏主题按钮：图标随当前选择变化，跟随系统时用显示器图标 */
export function ThemeMenu() {
  const { t } = useTranslation()
  const { theme, resolved, setTheme } = useTheme()
  const next = nextThemeChoice(theme)
  const currentLabel = t(THEME_LABEL_KEY[theme])
  const nextLabel = t(THEME_LABEL_KEY[next])
  const statusText = t('theme.cycleLabel', { current: currentLabel, next: nextLabel })

  const Icon = theme === 'system' ? MonitorIcon : resolved === 'dark' ? MoonIcon : SunIcon

  const cycle = () => setTheme(next)

  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      aria-label={statusText}
      title={statusText}
      onClick={cycle}
    >
      <Icon />
    </Button>
  )
}