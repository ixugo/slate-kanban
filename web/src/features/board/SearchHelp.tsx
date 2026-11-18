// 搜索语法说明：条目与后端 card_filter.go 一致，示例可点进搜索框。

import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { HelpCircleIcon } from 'lucide-react'

const TERM_KEYS = ['tag', 'col', 'priority', 'is', 'has', 'no', 'due', 'text'] as const
const EXAMPLE_KEYS = ['openBugs', 'overdue', 'noDue'] as const

interface SearchHelpProps {
  onExample: (q: string) => void
}

/** 搜索框旁的语法帮助弹层 */
export function SearchHelp({ onExample }: SearchHelpProps) {
  const { t } = useTranslation()
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button type="button" variant="ghost" size="icon-sm" aria-label={t('search.syntax')}>
          <HelpCircleIcon />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80 space-y-3 text-ui" align="end">
        <p className="font-medium">{t('search.syntaxTitle')}</p>
        <ul className="space-y-1 text-muted-foreground">
          {TERM_KEYS.map((key) => (
            <li key={key}>{t(`search.terms.${key}`)}</li>
          ))}
        </ul>
        <div className="flex flex-wrap gap-1">
          {EXAMPLE_KEYS.map((key) => {
            const ex = t(`search.examples.${key}`)
            return (
              <Button
                key={key}
                type="button"
                variant="secondary"
                size="sm"
                className="font-mono text-[11px]"
                onClick={() => onExample(ex)}
              >
                {ex}
              </Button>
            )
          })}
        </div>
      </PopoverContent>
    </Popover>
  )
}