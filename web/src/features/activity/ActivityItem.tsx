// 活动流里的一条：谁、做了什么、涉及哪个任务、多久以前。

import { BotIcon, UserIcon, ZapIcon } from 'lucide-react'
import { Link, useSearchParams } from 'react-router-dom'
import { useTranslation } from '@/i18n/react'
import { useLanguage } from '@/i18n/useLanguage'
import type { ActorKind, BoardEvent, CardView, Engineer } from '@/lib/api/types'
import { formatDateTime, formatRelative } from '@/lib/format'
import { describeEvent, eventActorLabel, eventCard } from './describeEvent'
import { ViewChangeLink } from './ViewChangeLink'
import { CardReference } from '@/features/card/CardReference'
import { EventPreviousValues } from './EventPreviousValues'
import { displayKey } from '@/lib/cardKey'

interface ActivityItemProps {
  event: BoardEvent
  /** 看板上现有的任务，用来取任务现在的标题并判断能不能点开 */
  cardsById: ReadonlyMap<number, CardView>
  engineers: readonly Engineer[]
  /** 点了任务链接后调用，让抽屉关掉、露出任务弹窗 */
  onOpenCard: () => void
}

const ACTOR_ICONS: Record<ActorKind, typeof UserIcon> = { human: UserIcon, agent: BotIcon, system: ZapIcon }

/** 活动流里的一条事件 */
export function ActivityItem({ event, cardsById, engineers, onOpenCard }: ActivityItemProps) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const [params, setParams] = useSearchParams()
  const { text, detail, descriptionChangeId, previousValues } = describeEvent(event, t, engineers)
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
          <span className="font-medium">{eventActorLabel(event, engineers, t)}</span>{' '}
          <span className="text-muted-foreground">{text}</span>
          {/* 任务已经删除时，它的变更记录也一并删除了，所以不再给「查看变更」 */}
          {descriptionChangeId !== undefined && card?.id != null && <ViewChangeLink cardId={card.id} changeId={descriptionChangeId} />}
        </p>
        {detail !== '' && <p className="line-clamp-3 text-xs break-words text-muted-foreground">{detail}</p>}
        <EventPreviousValues
          values={previousValues}
          renderReference={(id, number) => {
            const target = cardsById.get(id)
            const label = `#${number}`
            if (!target) return label
            return (
              <CardReference label={label} card={target} onOpen={() => {
                setParams((prev) => {
                  const params = new URLSearchParams(prev)
                  params.set('card', String(target.id))
                  return params
                })
                onOpenCard()
              }} />
            )
          }}
        />
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
