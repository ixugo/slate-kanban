// 活动流里的一条：谁、做了什么、涉及哪张卡片、多久以前。

import { BotIcon, UserIcon, ZapIcon } from 'lucide-react'
import { Link, useSearchParams } from 'react-router-dom'
import { useTranslation } from '@/i18n/react'
import { useLanguage } from '@/i18n/useLanguage'
import type { ActorKind, BoardEvent, CardView } from '@/lib/api/types'
import { formatDateTime, formatRelative } from '@/lib/format'
import { actorLabel, describeEvent, eventCard } from './describeEvent'
import { displayKey } from '@/lib/cardKey'

interface ActivityItemProps {
  event: BoardEvent
  /** 看板上现有的卡片，用来取卡片现在的标题并判断能不能点开 */
  cardsById: ReadonlyMap<number, CardView>
  /** 点了卡片链接后调用，让抽屉关掉、露出卡片弹窗 */
  onOpenCard: () => void
}

const ACTOR_ICONS: Record<ActorKind, typeof UserIcon> = { human: UserIcon, agent: BotIcon, system: ZapIcon }

/** 活动流里的一条事件 */
export function ActivityItem({ event, cardsById, onOpenCard }: ActivityItemProps) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const [params] = useSearchParams()
  const { text, detail } = describeEvent(event, t)
  const card = eventCard(event, cardsById)
  const Icon = ACTOR_ICONS[event.actor_kind] ?? ZapIcon
  // 沿用地址里已有的其他参数，只替换 card
  const next = new URLSearchParams(params)
  if (card?.id != null) next.set('card', String(card.id))

  return (
    <li className="flex gap-3 py-3">
      <span className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full bg-secondary text-muted-foreground">
        <Icon className="size-3.5" aria-hidden />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <p className="text-sm leading-snug break-words">
          <span className="font-medium">{actorLabel({ kind: event.actor_kind, name: event.actor }, t)}</span>{' '}
          <span className="text-muted-foreground">{text}</span>
        </p>
        {detail !== '' && <p className="line-clamp-3 text-xs break-words text-muted-foreground">{detail}</p>}
        {card && (
          <p className="truncate text-xs">
            {card.id === null ? (
              <span className="text-muted-foreground">{`${displayKey(card.key)} ${card.title}`.trim()}</span>
            ) : (
              <Link
                to={{ search: next.toString() }}
                onClick={onOpenCard}
                className="rounded-sm font-medium text-foreground underline-offset-2 hover:underline focus-visible:ring-[3px] focus-visible:ring-ring/40 focus-visible:outline-none"
              >
                {`${displayKey(card.key)} ${card.title}`.trim()}
              </Link>
            )}
          </p>
        )}
        <time dateTime={event.created_at} title={formatDateTime(event.created_at, lang)} className="font-mono text-[0.6875rem] text-muted-foreground">
          {formatRelative(event.created_at, lang)}
        </time>
      </div>
    </li>
  )
}
