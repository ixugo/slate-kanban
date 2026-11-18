// 设置页「自动化」：规则列表、启用开关、编辑与删除。

import { PencilIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from '@/i18n/react'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import type { Column, Rule } from '@/lib/api/types'
import { toast } from '@/lib/toast'
import { useErrorToast } from '@/lib/useErrorToast'
import { RuleEditorDialog } from './RuleEditorDialog'
import { RuleSummaryText } from './RuleSummaryText'
import { useSettingsMutations } from './useSettingsMutations'

interface Props {
  boardId: number
  columns: Column[]
  rules: Rule[]
}

/** 自动化规则区块 */
export function AutomationSection({ boardId, columns, rules }: Props) {
  const { t } = useTranslation()
  const toastError = useErrorToast()
  const { toggleRule, deleteRule } = useSettingsMutations(boardId)
  const [editing, setEditing] = useState<Rule | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Rule | null>(null)
  const sorted = [...rules].sort((a, b) => a.sort - b.sort || a.id - b.id)

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm text-muted-foreground">{t('settings.rules.subtitle')}</p>
        <Button size="sm" onClick={() => setEditing('new')}>
          <PlusIcon />
          {t('settings.rules.add')}
        </Button>
      </div>
      {sorted.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('settings.rules.empty')}</p>
      ) : (
        <ul className="divide-y divide-border border border-border">
          {sorted.map((rule) => (
            <li key={rule.id} className="space-y-2 p-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-medium">{rule.name}</span>
                <div className="flex items-center gap-2">
                  <Switch
                    checked={rule.enabled}
                    aria-label={t('settings.rules.enabled')}
                    onCheckedChange={(v) => toggleRule.mutate({ ruleId: rule.id, enabled: v }, { onError: toastError })}
                  />
                  <Button type="button" size="icon-sm" variant="ghost" aria-label={t('common.edit')} onClick={() => setEditing(rule)}>
                    <PencilIcon />
                  </Button>
                  <Button type="button" size="icon-sm" variant="ghost" aria-label={t('common.delete')} onClick={() => setDeleting(rule)}>
                    <Trash2Icon />
                  </Button>
                </div>
              </div>
              <RuleSummaryText rule={rule} columns={columns} />
            </li>
          ))}
        </ul>
      )}
      <RuleEditorDialog
        key={editing === 'new' ? 'new' : editing?.id ?? 'closed'}
        boardId={boardId}
        columns={columns}
        rule={editing && editing !== 'new' ? editing : undefined}
        open={editing !== null}
        onOpenChange={(v) => !v && setEditing(null)}
      />
      <AlertDialog open={deleting !== null} onOpenChange={(v) => !v && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('settings.rules.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('settings.rules.deleteBody')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <Button
              variant="destructive"
              disabled={deleteRule.isPending}
              onClick={() => {
                if (!deleting) return
                deleteRule.mutate(deleting.id, {
                  onSuccess: () => {
                    toast.success(t('settings.rules.deletedToast'))
                    setDeleting(null)
                  },
                  onError: toastError,
                })
              }}
            >
              {t('common.delete')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}