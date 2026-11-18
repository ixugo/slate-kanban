// 语言选择菜单：五种语言各用自己的文字显示，不管当前界面是什么语言都认得出来。

import { GlobeIcon } from 'lucide-react'
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
import { LANGUAGES } from '@/i18n/languages'
import { useLanguage } from '@/i18n/useLanguage'

/** 顶栏上的语言菜单；窄屏只显示图标 */
export function LanguageMenu() {
  const { t } = useTranslation()
  const { lang, setLang } = useLanguage()
  const current = LANGUAGES.find((l) => l.code === lang)

  // 菜单传回的是字符串，只接受五种语言之一，其余忽略
  const choose = (value: string) => {
    const next = LANGUAGES.find((l) => l.code === value)
    if (next) setLang(next.code)
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" aria-label={t('language.label')}>
          <GlobeIcon />
          <span className="hidden md:inline" lang={lang}>
            {current?.label}
          </span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>{t('language.label')}</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={lang} onValueChange={choose}>
          {LANGUAGES.map((l) => (
            <DropdownMenuRadioItem key={l.code} value={l.code} lang={l.code}>
              {l.label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
