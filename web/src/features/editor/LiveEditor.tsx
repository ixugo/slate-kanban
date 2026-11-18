// 带工具栏的实时渲染编辑器（React 外壳）：输入 Markdown 立即渲染，支持常用快捷键。
// 内核在 LiveView.ts；这里只管创建它、把文字同步给父组件，以及工具栏。
// 本文件作为懒加载块，卡片弹窗真正要编辑时才下载，不占首屏体积。

import { useEffect, useRef, useState, type FocusEvent } from 'react'
import {
  Bold,
  CircleHelp,
  Code,
  Italic,
  Link,
  List,
  ListChecks,
  ListOrdered,
  Quote,
  SquareCode,
  Strikethrough,
  Type,
  Underline,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { cn } from '@/lib/utils'
import { uploadAttachment } from '@/lib/api/endpoints'
import { useErrorToast } from '@/lib/useErrorToast'
import { LiveView } from './LiveView'
import { COMMANDS, type Command } from './keymap'
import './editor.css'

interface LiveEditorProps {
  value: string
  onChange: (value: string) => void
  /** 文字长度上限 */
  max: number
  placeholder: string
  /** 给读屏软件的名称 */
  label: string
  /** Ctrl/Cmd+Enter */
  onSubmit?: () => void
  /** Esc */
  onCancel?: () => void
  /** 挂载后自动获得焦点，光标在末尾 */
  autoFocus?: boolean
  /** 紧凑模式：编辑区更矮（评论用） */
  compact?: boolean
  /** 图片上传到哪个看板；不提供时不处理粘贴图片 */
  boardId?: number
  /** 只有编辑区获得焦点时才显示工具栏，没焦点时收成一行（评论框用，和描述的查看状态一样清爽） */
  toolbarOnFocus?: boolean
}

/** 工具栏上的一个按钮：图标、名称（同时作为悬停提示和读屏名称）和对应命令 */
interface Tool {
  id: string
  icon: LucideIcon
  command: Command
}

const TOOLS: readonly (readonly Tool[])[] = [
  [
    { id: 'bold', icon: Bold, command: COMMANDS.bold },
    { id: 'italic', icon: Italic, command: COMMANDS.italic },
    { id: 'underline', icon: Underline, command: COMMANDS.underline },
    { id: 'strike', icon: Strikethrough, command: COMMANDS.strike },
  ],
  [
    { id: 'bullet', icon: List, command: COMMANDS.bullet },
    { id: 'ordered', icon: ListOrdered, command: COMMANDS.ordered },
    { id: 'task', icon: ListChecks, command: COMMANDS.task },
  ],
  [
    { id: 'quote', icon: Quote, command: COMMANDS.quote },
    { id: 'code', icon: Code, command: COMMANDS.code },
    { id: 'codeBlock', icon: SquareCode, command: COMMANDS.codeBlock },
    { id: 'link', icon: Link, command: COMMANDS.link },
  ],
]

/** 快捷键提示里的主修饰键写法 */
const MOD = 'Ctrl/⌘'

/** 快捷键速查表：[按键, 说明的文案键] */
const SHORTCUTS = [
  [`${MOD} B`, 'bold'],
  [`${MOD} I`, 'italic'],
  [`${MOD} U`, 'underline'],
  [`${MOD} Shift X`, 'strike'],
  [`${MOD} E`, 'code'],
  [`${MOD} K`, 'link'],
  [`${MOD} Alt 1–6`, 'heading'],
  [`${MOD} Shift 7`, 'ordered'],
  [`${MOD} Shift 8`, 'bullet'],
  [`${MOD} Shift 9`, 'task'],
  [`${MOD} Shift .`, 'quote'],
  [`${MOD} Alt C`, 'codeBlock'],
  ['Tab / Shift Tab', 'indent'],
  [`${MOD} Z`, 'undo'],
  [`${MOD} Shift Z`, 'redo'],
  [`${MOD} Enter`, 'submit'],
] as const

/** 工具栏按钮共用的样式：触屏上放大到 40px，方便点 */
const TOOL_BUTTON = 'size-8 pointer-coarse:size-10'

/** 实时渲染编辑器 */
export default function LiveEditor({ value, onChange, max, placeholder, label, onSubmit, onCancel, autoFocus, compact, boardId, toolbarOnFocus }: LiveEditorProps) {
  const { t } = useTranslation()
  const rootRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<LiveView | null>(null)
  const [headingOpen, setHeadingOpen] = useState(false)
  const [uploading, setUploading] = useState(0)
  const [focused, setFocused] = useState(false)
  const showError = useErrorToast()
  // 回调放进 ref：父组件每次渲染都会生成新函数，不能因此重建编辑器
  const latest = useRef({ onChange, onSubmit, onCancel, showError })
  useEffect(() => {
    latest.current = { onChange, onSubmit, onCancel, showError }
  })

  /** 把粘贴的图片按顺序上传，每传完一张就在光标处插入图片语法；失败的弹提示，不影响其余的 */
  const pasteImages = async (target: number, files: File[]) => {
    for (const file of files) {
      setUploading((n) => n + 1)
      try {
        const { url } = await uploadAttachment(target, file)
        viewRef.current?.insert(`![image](${url})`)
      } catch (err) {
        latest.current.showError(err)
      } finally {
        setUploading((n) => n - 1)
      }
    }
  }

  useEffect(() => {
    const root = rootRef.current
    if (!root) return
    const view = new LiveView(root, value, {
      max,
      onChange: (v) => latest.current.onChange(v),
      onSubmit: () => latest.current.onSubmit?.(),
      onCancel: latest.current.onCancel ? () => latest.current.onCancel?.() : undefined,
      onPasteImages: boardId === undefined ? undefined : (files) => void pasteImages(boardId, files),
    })
    viewRef.current = view
    if (autoFocus) view.focus()
    return () => {
      view.destroy()
      viewRef.current = null
    }
    // 编辑器只在挂载时创建一次；之后的 value 变化走下面的同步
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    viewRef.current?.setValue(value)
  }, [value])

  /** 执行一条命令后把焦点还给编辑区 */
  const run = (command: Command) => {
    viewRef.current?.exec(command)
    viewRef.current?.focus()
  }

  /** 焦点离开整个编辑器（含弹出的标题菜单和帮助气泡，它们渲染在编辑器外面）才算失焦 */
  const onBlur = (e: FocusEvent<HTMLDivElement>) => {
    const next = e.relatedTarget as HTMLElement | null
    if (next && (e.currentTarget.contains(next) || next.closest('[data-slot="popover-content"]'))) return
    setFocused(false)
  }
  const collapsed = Boolean(toolbarOnFocus) && !focused && !headingOpen

  return (
    <div
      className="overflow-hidden rounded-xl border border-input bg-field transition-[border-color,box-shadow] duration-200 focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/25"
      onFocus={() => setFocused(true)}
      onBlur={onBlur}
    >
      {!collapsed && (
      <div
        role="toolbar"
        aria-label={t('editor.toolbar')}
        className="flex items-center gap-0.5 overflow-x-auto border-b border-border px-1.5 py-1 [scrollbar-width:none]"
        // 点工具栏不能让编辑区失去焦点，否则选区就丢了
        onMouseDown={(e) => e.preventDefault()}
      >
        <Popover open={headingOpen} onOpenChange={setHeadingOpen}>
          <PopoverTrigger asChild>
            <Button type="button" variant="ghost" size="icon" className={TOOL_BUTTON} aria-label={t('editor.heading')} title={t('editor.heading')}>
              <Type />
            </Button>
          </PopoverTrigger>
          <PopoverContent align="start" className="w-44 gap-0.5 p-1.5" onCloseAutoFocus={(e) => e.preventDefault()}>
            {[1, 2, 3, 0].map((level) => (
              <Button
                key={level}
                type="button"
                variant="ghost"
                size="sm"
                className="h-8 justify-start pointer-coarse:h-10"
                onClick={() => {
                  setHeadingOpen(false)
                  run(COMMANDS.heading(level))
                }}
              >
                {t(level === 0 ? 'editor.paragraph' : (`editor.h${level}` as 'editor.h1'))}
              </Button>
            ))}
          </PopoverContent>
        </Popover>
        {TOOLS.map((group, i) => (
          <div key={i} className="flex items-center gap-0.5 border-l border-border/70 pl-0.5 first:border-l-0 first:pl-0">
            {group.map(({ id, icon: Icon, command }) => {
              const name = t(`editor.${id}` as 'editor.bold')
              return (
                <Button key={id} type="button" variant="ghost" size="icon" className={TOOL_BUTTON} aria-label={name} title={name} onClick={() => run(command)}>
                  <Icon />
                </Button>
              )
            })}
          </div>
        ))}
        <Popover>
          <PopoverTrigger asChild>
            <Button type="button" variant="ghost" size="icon" className={cn(TOOL_BUTTON, 'ml-auto')} aria-label={t('editor.help')} title={t('editor.help')}>
              <CircleHelp />
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" className="max-h-[60dvh] w-[min(20rem,calc(100vw-2rem))] overflow-y-auto" onCloseAutoFocus={(e) => e.preventDefault()}>
            <p className="text-muted-foreground">{t('editor.intro')}</p>
            <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5">
              {SHORTCUTS.map(([keys, name]) => (
                <div key={name} className="contents">
                  <dt>
                    <kbd className="rounded-md bg-secondary px-1.5 py-0.5 font-mono text-xs whitespace-nowrap">{keys}</kbd>
                  </dt>
                  <dd className="text-muted-foreground">{t(`editor.${name}` as 'editor.bold')}</dd>
                </div>
              ))}
            </dl>
          </PopoverContent>
        </Popover>
      </div>
      )}
      <div
        ref={rootRef}
        data-placeholder={placeholder}
        aria-label={label}
        aria-placeholder={placeholder}
        className={cn('md-editor max-h-[50dvh] overflow-y-auto px-3 py-2.5 text-body', compact ? (collapsed ? 'min-h-10' : 'min-h-16') : 'min-h-32')}
      />
      {uploading > 0 && (
        <p role="status" className="border-t border-border px-3 py-1.5 text-xs text-muted-foreground">
          {t('editor.uploading')}
        </p>
      )}
    </div>
  )
}
