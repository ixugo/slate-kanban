// 主题选择菜单：浅色、深色、跟随系统；按钮上的图标是当前实际生效的主题。

import { MonitorIcon, MoonIcon, SunIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useTheme, type ThemeChoice } from '@/lib/theme-context'

/** 三个选项的值、文案键和图标 */
const OPTIONS = [
  { value: 'light', key: 'theme.light', Icon: SunIcon },
  { value: 'dark', key: 'theme.dark', Icon: MoonIcon },
  { value: 'system', key: 'theme.system', Icon: MonitorIcon },
] as const

/** 顶栏上的主题菜单 */
export function ThemeMenu() {
  const { t } = useTranslation()
  const { theme, resolved, setTheme } = useTheme()
  const CurrentIcon = resolved === 'dark' ? MoonIcon : SunIcon

  // 菜单传回的是字符串，只接受三种主题之一
  const choose = (value: string) => {
    const next = OPTIONS.find((o) => o.value === value)
    if (next) setTheme(next.value satisfies ThemeChoice)
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t('theme.label')}>
          <CurrentIcon />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>{t('theme.label')}</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={theme} onValueChange={choose}>
          {OPTIONS.map(({ value, key, Icon }) => (
            <DropdownMenuRadioItem key={value} value={value}>
              <Icon />
              {t(key)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
