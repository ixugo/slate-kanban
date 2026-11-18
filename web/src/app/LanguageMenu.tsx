// 语言选择：原生 select，五种语言 option 自带 lang；窄屏用透明 select 盖住地球图标扩大点击区域。

import { ChevronDownIcon, GlobeIcon } from 'lucide-react'
import type { ChangeEvent } from 'react'
import { cn } from 'cn'
import { useTranslation } from '@/i18n/react'
import { LANGUAGES } from '@/i18n/languages'
import { useLanguage } from '@/i18n/useLanguage'

/** 顶栏语言下拉；样式与 ghost 按钮一致 */
export function LanguageMenu() {
  const { t } = useTranslation()
  const { lang, setLang } = useLanguage()

  const onChange = (event: ChangeEvent<HTMLSelectElement>) => {
    const next = LANGUAGES.find((l) => l.code === event.target.value)
    if (next) setLang(next.code)
  }

  return (
    <div
      className={cn(
        'group/lang relative inline-flex h-7 min-w-7 items-center',
        'rounded-none border border-transparent focus-within:border-ring focus-within:ring-1 focus-within:ring-ring/50',
      )}
    >
      <GlobeIcon
        className="pointer-events-none absolute left-1.5 z-10 size-3.5 text-muted-foreground md:static md:ml-1.5 md:shrink-0"
        aria-hidden
      />
      <select
        aria-label={t('language.label')}
        value={lang}
        onChange={onChange}
        className={cn(
          'h-7 w-7 cursor-pointer appearance-none rounded-none border-0 bg-transparent p-0 text-xs outline-none',
          'absolute inset-0 opacity-0 md:static md:w-auto md:min-w-[6.5rem] md:max-w-[9rem] md:cursor-pointer md:truncate md:pl-7 md:pr-6 md:opacity-100',
          'md:hover:bg-muted md:dark:hover:bg-muted/50',
        )}
      >
        {LANGUAGES.map((l) => (
          <option key={l.code} value={l.code} lang={l.code}>
            {l.label}
          </option>
        ))}
      </select>
      <ChevronDownIcon
        className="pointer-events-none absolute right-1 hidden size-3.5 text-muted-foreground md:block"
        aria-hidden
      />
    </div>
  )
}