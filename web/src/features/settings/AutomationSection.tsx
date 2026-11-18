// 设置页「自动化」：推荐规则一键添加、我的规则列表与简化编辑器。

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
import { autoNameFromFormState, isAutoRuleName } from './ruleAutoName'
import { listMissingRecommendedPresets } from './ruleMatch'
import { buildRulePreset, presetToCreateInput, type RulePresetId } from './rulePresets'
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
  const { toggleRule, deleteRule, createRule } = useSettingsMutations(boardId)
  const [editing, setEditing] = useState<Rule | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Rule | null>(null)
  const sorted = [...rules].sort((a, b) => a.sort - b.sort || a.id - b.id)
  const recommended = listMissingRecommendedPresets(columns, rules)

  /** 推荐规则：点添加即 POST，名称用自动命名 */
  const addRecommended = (presetId: RulePresetId) => {
    const form = buildRulePreset(presetId, columns)
    const name = autoNameFromFormState(form.trigger, form.columnId, form.actions, columns, t)
    const input = presetToCreateInput(presetId, columns, name)
    if (!input) return
    createRule.mutate(input, {
      onSuccess: () => toast.success(t('settings.rules.createdToast')),
      onError: toastError,
    })
  }

  return (
    <div className="space-y-6">
      <p className="text-sm text-muted-foreground">{t('settings.rules.subtitle')}</p>

      {recommended.length > 0 && (
        <section className="space-y-2" aria-labelledby="rules-recommended-heading">
          <h3 id="rules-recommended-heading" className="text-sm font-medium">
            {t('settings.rules.recommended')}
          </h3>
          <ul className="divide-y divide-border border border-border">
            {recommended.map((presetId) => {
              const form = buildRulePreset(presetId, columns)
              const label = autoNameFromFormState(form.trigger, form.columnId, form.actions, columns, t)
              return (
                <li key={presetId} className="flex flex-wrap items-center justify-between gap-2 p-3">
                  <span className="text-sm">{label}</span>
                  <Button
                    type="button"
                    size="sm"
                    variant="secondary"
                    aria-label={`${t('settings.rules.recommendedAdd')} ${label}`}
                    disabled={createRule.isPending}
                    onClick={() => addRecommended(presetId)}
                  >
                    {t('settings.rules.recommendedAdd')}
                  </Button>
                </li>
              )
            })}
          </ul>
        </section>
      )}

      <section className="space-y-3" aria-labelledby="rules-my-heading">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h3 id="rules-my-heading" className="text-sm font-medium">
            {t('settings.rules.myRules')}
          </h3>
          <Button size="sm" onClick={() => setEditing('new')}>
            <PlusIcon />
            {t('settings.rules.add')}
          </Button>
        </div>
        {sorted.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t('settings.rules.empty')}</p>
        ) : (
          <ul className="divide-y divide-border border border-border">
            {sorted.map((rule) => {
              const showSummary = !isAutoRuleName(rule, columns, t)
              return (
                <li key={rule.id} className="space-y-1 p-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="font-medium">{rule.name}</span>
                    <div className="flex items-center gap-2">
                      <Switch
                        checked={rule.enabled}
                        aria-label={`${t('settings.rules.enabled')}: ${rule.name}`}
                        onCheckedChange={(v) => toggleRule.mutate({ ruleId: rule.id, enabled: v }, { onError: toastError })}
                      />
                      <Button
                        type="button"
                        size="icon-sm"
                        variant="ghost"
                        aria-label={t('common.edit')}
                        onClick={() => setEditing(rule)}
                      >
                        <PencilIcon />
                      </Button>
                      <Button
                        type="button"
                        size="icon-sm"
                        variant="ghost"
                        aria-label={t('common.delete')}
                        onClick={() => setDeleting(rule)}
                      >
                        <Trash2Icon />
                      </Button>
                    </div>
                  </div>
                  {showSummary && (
                    <p className="text-sm">
                      <RuleSummaryText rule={rule} columns={columns} />
                    </p>
                  )}
                </li>
              )
            })}
          </ul>
        )}
      </section>

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