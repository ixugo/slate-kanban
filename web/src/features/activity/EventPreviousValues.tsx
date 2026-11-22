import { Fragment, type ReactNode } from 'react'
import { useTranslation } from '@/i18n/react'
import type { PreviousValue } from './describeEvent'

interface EventPreviousValuesProps {
  values?: PreviousValue[]
  renderReference: (id: number, number: number) => ReactNode
}

/** 旧值与活动标题分开，合并多个字段时仍能对应到各自的值。 */
export function EventPreviousValues({ values, renderReference }: EventPreviousValuesProps) {
  const { t } = useTranslation()
  if (!values?.length) return null
  return (
    <dl className="space-y-0.5 text-xs text-muted-foreground">
      {values.map((value) => (
        <div key={value.field} className="flex flex-wrap items-baseline gap-x-1">
          <dt>{t('activity.previous.label')}{values.length > 1 && ` · ${value.label}`}</dt>
          <dd className="min-w-0 break-all text-foreground">
            {'text' in value
              ? value.text || t('common.none')
              : value.references.length > 0
                ? <span className="inline-flex flex-wrap gap-x-2 font-mono">
                    {value.references.map(({ id, number }) => <Fragment key={id}>{renderReference(id, number)}</Fragment>)}
                  </span>
                : t('common.none')}
          </dd>
        </div>
      ))}
    </dl>
  )
}
