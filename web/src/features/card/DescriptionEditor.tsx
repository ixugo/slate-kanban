// 卡片描述：平时是渲染好的 Markdown（点一下进入编辑，任务框可直接勾选）；
// 编辑时是实时渲染编辑器，Ctrl/Cmd+Enter 保存，Esc 取消。

import { useState, type MouseEvent } from 'react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { LazyEditor } from '@/features/editor/LazyEditor'
import { toggleTaskAt } from '@/lib/md/blocks'
import { Markdown } from '@/lib/markdown'
import { CARD_DESC_MAX } from '@/lib/limits'

interface DescriptionEditorProps {
  value: string
  onSave: (next: string) => void
  busy?: boolean
}

/** 描述的查看与编辑 */
export function DescriptionEditor({ value, onSave, busy }: DescriptionEditorProps) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(value)

  const startEdit = () => {
    setDraft(value)
    setEditing(true)
  }

  const cancel = () => {
    setDraft(value)
    setEditing(false)
  }

  const save = () => {
    if (draft !== value) onSave(draft)
    setEditing(false)
  }

  /** 点在链接或任务框上不进入编辑，其余地方点一下就开始编辑 */
  const onViewClick = (e: MouseEvent<HTMLDivElement>) => {
    if ((e.target as HTMLElement).closest('a, input, button')) return
    if (window.getSelection()?.toString()) return
    startEdit()
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-body font-semibold tracking-tight">{t('card.field.description')}</h3>
        {!editing && (
          <Button type="button" size="sm" variant="secondary" onClick={startEdit}>
            {t('card.descEdit')}
          </Button>
        )}
      </div>
      {editing ? (
        <>
          <LazyEditor
            value={draft}
            onChange={setDraft}
            max={CARD_DESC_MAX}
            placeholder={t('card.descPlaceholder')}
            label={t('card.field.description')}
            onSubmit={save}
            onCancel={cancel}
            autoFocus
          />
          <div className="flex gap-2">
            <Button type="button" size="sm" disabled={busy} onClick={save}>
              {t('common.save')}
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={cancel}>
              {t('common.cancel')}
            </Button>
          </div>
        </>
      ) : value ? (
        <div className="-mx-2.5 cursor-text rounded-xl px-2.5 py-1.5 transition-colors hover:bg-secondary/50" onClick={onViewClick}>
          <Markdown source={value} onToggleTask={busy ? undefined : (line) => onSave(toggleTaskAt(value, line))} />
        </div>
      ) : (
        <button
          type="button"
          className="block min-h-16 w-full cursor-text rounded-xl bg-secondary/50 px-3 py-2.5 text-left text-ui text-muted-foreground transition-colors hover:bg-secondary"
          onClick={startEdit}
        >
          {t('card.descPlaceholder')}
        </button>
      )}
    </div>
  )
}
