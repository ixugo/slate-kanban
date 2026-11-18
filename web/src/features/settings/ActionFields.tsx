// 规则编辑器里单个动作类型的参数字段。

import type { ReactNode } from 'react'
import { useTranslation } from '@/i18n/react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import type { Column } from '@/lib/api/types'
import {
  CARD_PRIORITY_MAX,
  CARD_TAG_MAX,
  RULE_COMMENT_MAX,
  RULE_DUE_DAYS_MAX,
} from '@/lib/limits'
import type { ActionFormRow } from './ruleForm'

interface Props {
  row: ActionFormRow
  columns: Column[]
  onChange: (row: ActionFormRow) => void
}

/** 按动作类型渲染对应表单控件 */
export function ActionFields({ row, columns, onChange }: Props) {
  const { t } = useTranslation()
  switch (row.type) {
    case 'set_started_at':
    case 'set_finished_at':
      return (
        <MotionField
          label={t('settings.rules.actions.mode')}
          control={
            <Select value={row.mode} onValueChange={(v) => onChange({ ...row, mode: v as ActionFormRow['mode'] })}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="if_empty">{t('settings.rules.actions.modeIfEmpty')}</SelectItem>
                <SelectItem value="always">{t('settings.rules.actions.modeAlways')}</SelectItem>
              </SelectContent>
            </Select>
          }
        />
      )
    case 'set_due':
      return (
        <MotionField
          label={t('settings.rules.actions.days')}
          control={
            <Input
              type="number"
              min={1}
              max={RULE_DUE_DAYS_MAX}
              value={row.days}
              onChange={(e) => onChange({ ...row, days: e.target.value })}
            />
          }
        />
      )
    case 'set_priority':
      return (
        <MotionField
          label={t('settings.rules.actions.priority')}
          control={
            <Select value={row.priority} onValueChange={(v) => onChange({ ...row, priority: v })}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {[0, 1, 2, 3].map((p) => (
                  <SelectItem key={p} value={String(p)} disabled={p > CARD_PRIORITY_MAX}>
                    {t(`board.priority.${p === 0 ? 'none' : p === 1 ? 'low' : p === 2 ? 'medium' : 'high'}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          }
        />
      )
    case 'add_tag':
    case 'remove_tag':
      return (
        <MotionField
          label={t('settings.rules.actions.tag')}
          control={
            <Input value={row.tag} maxLength={CARD_TAG_MAX} onChange={(e) => onChange({ ...row, tag: e.target.value })} />
          }
        />
      )
    case 'add_comment':
      return (
        <MotionField
          label={t('settings.rules.actions.comment')}
          control={
            <Textarea
              value={row.text}
              maxLength={RULE_COMMENT_MAX}
              rows={3}
              onChange={(e) => onChange({ ...row, text: e.target.value })}
            />
          }
        />
      )
    case 'move_to_column':
      return (
        <MotionField
          label={t('settings.rules.actions.targetColumn')}
          control={
            <Select
              value={row.columnId ? String(row.columnId) : ''}
              onValueChange={(v) => onChange({ ...row, columnId: Number(v) })}
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t('settings.rules.actions.pickColumn')} />
              </SelectTrigger>
              <SelectContent>
                {columns.map((c) => (
                  <SelectItem key={c.id} value={String(c.id)}>
                    {c.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          }
        />
      )
    case 'set_human_only':
      return (
        <div className="flex items-center justify-between gap-3">
          <Label htmlFor={`human-only-${row.id}`}>{t('settings.rules.actions.humanOnly')}</Label>
          <Switch
            id={`human-only-${row.id}`}
            checked={row.humanOnly}
            onCheckedChange={(v) => onChange({ ...row, humanOnly: v })}
          />
        </div>
      )
    default:
      return <p className="text-xs text-muted-foreground">{t('settings.rules.actions.noParams')}</p>
  }
}

/** 动作参数行的标签 + 控件布局 */
function MotionField({ label, control }: { label: string; control: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      {control}
    </div>
  )
}