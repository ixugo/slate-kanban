import { useState } from 'react'
import { CheckIcon, SearchIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import type { CardRef } from '@/lib/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { CARD_DEPS_MAX } from '@/lib/limits'
import { displayKey } from '@/lib/cardKey'

interface DependencyPickerProps {
  cardId: number
  selected: number[]
  options: CardRef[]
  onChange: (ids: number[]) => void
}

const MAX_RESULTS = 20
const MAX_TITLE_CHARACTERS = 10

function abbreviatedTitle(title: string) {
  const chars = Array.from(title)
  return chars.length <= MAX_TITLE_CHARACTERS ? title : chars.slice(0, MAX_TITLE_CHARACTERS).join('') + '…'
}

function dependencyNumber(card: CardRef) {
  return Number(displayKey(card.key).slice(1))
}

/** 在同项目的候选任务中持续勾选或取消依赖。 */
export function DependencyPicker({ cardId, selected, options, onChange }: DependencyPickerProps) {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')
  const search = query.trim().toLowerCase().replace(/^(\d+)$/, '#$1')
  const number = /^#(\d+)$/.exec(search)
  const matches = options
    .filter((card) => card.id !== cardId)
    .filter((card) => number ? dependencyNumber(card) === Number(number[1]) : card.title.toLowerCase().includes(search))
    .sort((a, b) => dependencyNumber(b) - dependencyNumber(a))
  // 前 20 条的位置不随勾选变化。保留其余已选项，方便不搜索也能取消。
  const results = matches.filter((card, index) => index < MAX_RESULTS || selected.includes(card.id))
  const atLimit = selected.length >= CARD_DEPS_MAX
  const toggle = (id: number) => {
    if (selected.includes(id)) onChange(selected.filter((dep) => dep !== id))
    else if (!atLimit) onChange([...selected, id])
  }

  return (
    <div className="flex min-h-0 flex-col gap-2">
      <div className="relative">
        <SearchIcon aria-hidden className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          aria-label={t('card.depSearch')}
          placeholder={t('card.depSearch')}
          className="pl-8"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            const first = results[0]
            if (event.key === 'Enter' && first && !event.nativeEvent.isComposing) {
              event.preventDefault()
              toggle(first.id)
            }
          }}
        />
      </div>
      <div className="h-48 overflow-y-auto">
        {results.length === 0 ? (
          <p role="status" className="px-2 py-1 text-xs text-muted-foreground">{t('card.depNoResults')}</p>
        ) : (
          <ul aria-label={t('card.depAdd')}>
            {results.map((card) => {
              const key = displayKey(card.key)
              const checked = selected.includes(card.id)
              return (
                <li key={card.id}>
                  <Button
                    type="button"
                    role="checkbox"
                    aria-checked={checked}
                    variant="ghost"
                    aria-label={`${key} ${card.title}`}
                    className="h-8 w-full min-w-0 justify-start rounded-md px-2 text-left aria-checked:bg-accent"
                    disabled={!checked && atLimit}
                    onClick={() => toggle(card.id)}
                  >
                    <span className="shrink-0 font-mono text-xs text-muted-foreground">{key}</span>
                    <span className="min-w-0 flex-1 truncate" title={card.title}>{abbreviatedTitle(card.title)}</span>
                    {checked && <CheckIcon aria-hidden />}
                  </Button>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </div>
  )
}
