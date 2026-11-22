import { useState } from 'react'
import { SearchIcon, XIcon } from 'lucide-react'
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

function DependencyTitle({ card }: { card: CardRef }) {
  return <span className="min-w-0 truncate" title={card.title}>{abbreviatedTitle(card.title)}</span>
}

/** 在同项目的候选任务中搜索并增删依赖。 */
export function DependencyPicker({ cardId, selected, options, onChange }: DependencyPickerProps) {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')
  const search = query.trim().toLowerCase()
  const number = /^#(\d+)$/.exec(search)
  const refs = new Map(options.map((card) => [card.id, card]))
  const results = options
    .filter((card) => card.id !== cardId && !selected.includes(card.id))
    .filter((card) => number ? dependencyNumber(card) === Number(number[1]) : card.title.toLowerCase().includes(search))
    .sort((a, b) => dependencyNumber(b) - dependencyNumber(a))
    .slice(0, MAX_RESULTS)
  const add = (id: number) => {
    onChange([...selected, id])
    setQuery('')
  }

  return (
    <div className="flex min-h-0 flex-col gap-2">
      {selected.length < CARD_DEPS_MAX && (
        <>
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
                  add(first.id)
                }
              }}
            />
          </div>
          {results.length === 0 ? (
            <p role="status" className="px-2 py-1 text-xs text-muted-foreground">{t('card.depNoResults')}</p>
          ) : (
            <ul aria-label={t('card.depAdd')} className="max-h-48 overflow-y-auto">
              {results.map((card) => (
                <li key={card.id}>
                  <Button
                    type="button"
                    variant="ghost"
                    aria-label={`${displayKey(card.key)} ${card.title}`}
                    className="h-8 w-full min-w-0 justify-start px-2 text-left"
                    onClick={() => add(card.id)}
                  >
                    <span className="shrink-0 font-mono text-xs text-muted-foreground">{displayKey(card.key)}</span>
                    <DependencyTitle card={card} />
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
      {selected.length > 0 && (
        <ul aria-label={t('card.field.deps')} className="max-h-36 overflow-y-auto border-t border-border/60 pt-1">
          {selected.map((id) => {
            const ref = refs.get(id)
            if (!ref) return null
            const key = displayKey(ref.key)
            return (
              <li key={id} className="flex min-w-0 items-center gap-2 px-2">
                <span className="shrink-0 font-mono text-xs text-muted-foreground">{key}</span>
                <DependencyTitle card={ref} />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-xs"
                  className="ml-auto shrink-0"
                  aria-label={`${t('common.delete')} ${key}`}
                  onClick={() => onChange(selected.filter((dep) => dep !== id))}
                >
                  <XIcon />
                </Button>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
