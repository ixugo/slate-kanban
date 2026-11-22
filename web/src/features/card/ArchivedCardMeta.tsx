import { Fragment } from 'react'
import { useTranslation } from '@/i18n/react'
import type { CardDetail, Engineer } from '@/lib/api/types'
import { formatDateTime } from '@/lib/format'
import type { CardReferenceRenderer } from '@/lib/markdown'
import { TagBadge } from '@/features/labels/TagBadge'
import { displayKey } from '@/lib/cardKey'
import { normalizeCardUrl } from './validateUrl'

const PRIORITIES = ['none', 'low', 'medium', 'high'] as const

interface ArchivedCardMetaProps {
  card: CardDetail
  engineers: Engineer[]
  lang: string
  renderCardReference: CardReferenceRenderer
}

/** 归档详情继续显示原任务属性，但不渲染编辑控件。 */
export function ArchivedCardMeta({ card, engineers, lang, renderCardReference }: ArchivedCardMetaProps) {
  const { t } = useTranslation()
  const engineer = engineers.find((item) => item.id === card.engineer_id)
  const url = normalizeCardUrl(card.url)
  const tags = card.archived_tags ?? []
  const rows = [
    { label: t('card.field.priority'), value: t(`board.priority.${PRIORITIES[card.priority] ?? 'none'}`) },
    { label: t('card.field.column'), value: card.column },
    { label: t('card.field.engineer'), value: card.archived_engineer ?? engineer?.name },
    { label: t('card.field.due'), value: formatDateTime(card.due_at, lang) },
    { label: t('card.field.branch'), value: card.branch },
    { label: t('card.field.url'), value: url.ok && url.value && <a className="underline" href={url.value} target="_blank" rel="noreferrer">{card.url}</a> },
    { label: t('card.field.tags'), value: tags.length > 0 && <div className="flex flex-wrap gap-1">{tags.map((tag) => <TagBadge key={tag.id} id={tag.id} archivedTag={tag} />)}</div> },
    { label: t('card.field.deps'), value: card.dependencies.length > 0 && (
      <ul className="space-y-1">
        {card.dependencies.map((dep) => {
          const label = displayKey(dep.key)
          const number = Number(label.slice(1))
          return (
            <li key={dep.id} className="flex min-w-0 flex-wrap items-baseline gap-x-2">
              {dep.missing ? <span className="font-mono text-muted-foreground">{label}</span> : renderCardReference(number, label)}
              <span>{dep.title}</span>
              {dep.missing && <span className="text-xs text-muted-foreground">{t('card.archive.missingDependency')}</span>}
            </li>
          )
        })}
      </ul>
    ) },
  ]
  return (
    <div className="space-y-2">
      {card.human_only && <p className="text-xs font-medium text-muted-foreground">{t('card.field.humanOnly')}</p>}
      <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm">
        {rows.filter(({ value }) => value).map(({ label, value }) => (
          <Fragment key={label}>
            <dt className="text-muted-foreground">{label}</dt>
            <dd className="min-w-0 break-words">{value}</dd>
          </Fragment>
        ))}
      </dl>
    </div>
  )
}
