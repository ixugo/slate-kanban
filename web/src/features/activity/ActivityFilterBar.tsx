// 活动流顶部的筛选条：事件类型（可多选）和操作者（单选），点一下立刻生效。

import { Chip, ChipGroup } from '@/components/ChipGroup'
import { Button } from '@/components/ui/button'
import { useTranslation } from '@/i18n/react'
import type { ActivityActor } from '@/lib/api/types'
import { actorLabel } from './describeEvent'
import {
  EVENT_GROUP_ORDER,
  NO_FILTERS,
  countFilters,
  isSameActor,
  toggleGroup,
  type ActivityFilters,
} from './filters'

interface ActivityFilterBarProps {
  filters: ActivityFilters
  onChange: (next: ActivityFilters) => void
  /** 这块看板的事件里出现过的操作者，由服务端给出 */
  actors: readonly ActivityActor[]
}

/** 事件类型和操作者的筛选条 */
export function ActivityFilterBar({ filters, onChange, actors }: ActivityFilterBarProps) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col gap-3 border-b px-4 py-3">
      <ChipGroup
        title={t('activity.typeTitle')}
        aside={
          countFilters(filters) > 0 ? (
            <Button type="button" variant="ghost" size="xs" className="-my-0.5" onClick={() => onChange(NO_FILTERS)}>
              {t('activity.clearFilters')}
            </Button>
          ) : undefined
        }
      >
        {EVENT_GROUP_ORDER.map((group) => (
          <Chip key={group} selected={filters.groups.includes(group)} onClick={() => onChange(toggleGroup(filters, group))}>
            {t(`activity.groups.${group}`)}
          </Chip>
        ))}
      </ChipGroup>
      {actors.length > 0 && (
        <ChipGroup title={t('activity.actorTitle')} radioGroup>
          <Chip radio selected={filters.actor === null} onClick={() => onChange({ ...filters, actor: null })}>
            {t('activity.anyone')}
          </Chip>
          {actors.map((actor) => (
            <Chip
              key={`${actor.kind}:${actor.name}`}
              radio
              selected={isSameActor(filters.actor, actor)}
              onClick={() => onChange({ ...filters, actor })}
            >
              {actorLabel(actor, t)}
            </Chip>
          ))}
        </ChipGroup>
      )}
    </div>
  )
}
