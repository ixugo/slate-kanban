// 卡片属性里的「工程师」小按钮：把卡片绑定给一位全栈工程师，不绑定则领取时随机分配。

import { useState } from 'react'
import { CheckIcon, UserIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import type { CardDetail, Engineer } from '@/lib/api/types'
import { cn } from '@/lib/utils'
import { isDevKind } from '@/features/settings/engineerKinds'

interface EngineerChipProps {
  card: CardDetail
  /** 看板的工程师名册，只有全栈工程师能绑定 */
  engineers: Engineer[]
  onPatch: (patch: Record<string, unknown>) => void
}

/** 工程师按钮：点开后在「随机分配」和各位全栈工程师里选一个；没有全栈工程师时不显示 */
export function EngineerChip({ card, engineers, onPatch }: EngineerChipProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const devs = engineers.filter((e) => isDevKind(e.kind))
  if (devs.length === 0 && card.engineer_id === 0) return null
  const bound = engineers.find((e) => e.id === card.engineer_id)

  const choose = (id: number) => {
    if (card.engineer_id !== id) onPatch({ engineer_id: id, version: card.version })
    setOpen(false)
  }
  const options: { id: number; label: string }[] = [{ id: 0, label: t('card.engineerAuto') }, ...devs.map((e) => ({ id: e.id, label: e.name }))]

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button type="button" variant="secondary" className={cn('h-8 max-w-full rounded-lg px-2.5 font-medium', bound && 'bg-accent')}>
          <UserIcon />
          <span className="truncate">{bound ? `${t('card.field.engineer')}: ${bound.name}` : t('card.field.engineer')}</span>
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-56 gap-0.5 p-1.5">
        <div role="radiogroup" aria-label={t('card.field.engineer')} className="flex flex-col gap-0.5">
          {options.map((o) => (
            <Button
              key={o.id}
              type="button"
              role="radio"
              aria-checked={card.engineer_id === o.id}
              variant="ghost"
              className="h-8 justify-between rounded-lg px-2.5"
              onClick={() => choose(o.id)}
            >
              <span className="truncate">{o.label}</span>
              {card.engineer_id === o.id && <CheckIcon />}
            </Button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}
