// 语言选择：原生 select，五种语言 option 自带 lang；窄屏用透明 select 盖住地球图标扩大点击区域。

import { ChevronsUpDownIcon, GlobeIcon } from 'lucide-react'
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
        'group/lang relative inline-flex h-8 min-w-8 items-center rounded-full transition-shadow duration-200',
        'focus-within:ring-[3px] focus-within:ring-ring/40',
      )}
    >
      <GlobeIcon
        className="pointer-events-none absolute left-2 z-10 size-4 text-muted-foreground md:static md:ml-3 md:shrink-0"
        aria-hidden
      />
      <select
        aria-label={t('language.label')}
        value={lang}
        onChange={onChange}
        className={cn(
          'h-8 w-8 cursor-pointer appearance-none rounded-full border-0 bg-transparent p-0 text-ui font-medium outline-none transition-colors duration-200',
          'absolute inset-0 opacity-0 md:static md:w-auto md:min-w-[7rem] md:max-w-[9.5rem] md:cursor-pointer md:truncate md:pl-2 md:pr-7 md:opacity-100',
          'md:-ml-5 md:pl-7 md:hover:bg-secondary',
        )}
      >
        {LANGUAGES.map((l) => (
          <option key={l.code} value={l.code} lang={l.code}>
            {l.label}
          </option>
        ))}
      </select>
      <ChevronsUpDownIcon
        className="pointer-events-none absolute right-2 hidden size-3.5 text-muted-foreground md:block"
        aria-hidden
      />
    </div>
  )
}