// 卡片描述：平时是渲染好的 Markdown（点一下进入编辑，任务框可直接勾选）；
// 编辑时是实时渲染编辑器：没有保存按钮，焦点离开或 Ctrl/Cmd+Enter 就保存，Esc 放弃修改。

import { useState, type FocusEvent, type MouseEvent } from 'react'
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
  /** 粘贴的图片上传到这个看板 */
  boardId: number
}

/** 描述的查看与编辑 */
export function DescriptionEditor({ value, onSave, busy, boardId }: DescriptionEditorProps) {
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

  /** 焦点离开编辑区就自动保存（没改动不请求）；焦点只是跑到帮助气泡等弹层里不算离开 */
  const onEditBlur = (e: FocusEvent<HTMLDivElement>) => {
    const next = e.relatedTarget as HTMLElement | null
    if (next && (e.currentTarget.contains(next) || next.closest('[data-slot="popover-content"]'))) return
    save()
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
        <div onBlur={onEditBlur}>
          <LazyEditor
            value={draft}
            onChange={setDraft}
            max={CARD_DESC_MAX}
            placeholder={t('card.descPlaceholder')}
            label={t('card.field.description')}
            onSubmit={save}
            onCancel={cancel}
            autoFocus
            boardId={boardId}
          />
        </div>
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
