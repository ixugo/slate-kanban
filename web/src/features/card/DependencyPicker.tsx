// 选择依赖卡片：只能选同看板其他卡，不能依赖自己。

import { useTranslation } from '@/i18n/react'
import type { CardRef } from '@/lib/api/types'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { XIcon } from 'lucide-react'
import { CARD_DEPS_MAX } from '@/lib/limits'

interface DependencyPickerProps {
  cardId: number
  selected: number[]
  options: CardRef[]
  onChange: (ids: number[]) => void
}

/** 多选依赖 */
export function DependencyPicker({ cardId, selected, options, onChange }: DependencyPickerProps) {
  const { t } = useTranslation()
  const available = options.filter((c) => c.id !== cardId && !selected.includes(c.id))

  const add = (id: string) => {
    const num = Number(id)
    if (selected.length >= CARD_DEPS_MAX) return
    onChange([...selected, num])
  }

  const remove = (id: number) => onChange(selected.filter((x) => x !== id))

  const labelOf = (id: number) => options.find((c) => c.id === id)

  return (
    <div className="space-y-2">
      {selected.length === 0 && <p className="text-sm text-muted-foreground">{t('card.depEmpty')}</p>}
      <ul className="space-y-1">
        {selected.map((id) => {
          const ref = labelOf(id)
          if (!ref) return null
          return (
            <li key={id} className="flex items-center gap-2 text-sm">
              <span className="font-mono text-xs text-muted-foreground">{ref.key}</span>
              <span className={ref.done ? 'text-muted-foreground line-through' : ''}>{ref.title}</span>
              <Button type="button" variant="ghost" size="icon-xs" aria-label={t('common.delete')} onClick={() => remove(id)}>
                <XIcon />
              </Button>
            </li>
          )
        })}
      </ul>
      {available.length > 0 && selected.length < CARD_DEPS_MAX && (
        <Select onValueChange={add}>
          <SelectTrigger className="h-8 w-full max-w-xs">
            <SelectValue placeholder={t('card.depAdd')} />
          </SelectTrigger>
          <SelectContent>
            {available.map((c) => (
              <SelectItem key={c.id} value={String(c.id)}>
                <span className="font-mono text-xs">{c.key}</span> {c.title}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </div>
  )
}