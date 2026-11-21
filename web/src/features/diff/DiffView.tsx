import { useEffect, useMemo, useRef, type JSX, type RefObject } from 'react'
import { cn } from '@/lib/utils'
import { diffLines, diffStats, type DiffRow, type Segment } from './lineDiff'

export interface DiffViewProps {
  oldText: string
  newText: string
  /** 左栏标题，由调用方传入（不做 i18n） */
  oldTitle: string
  /** 右栏标题 */
  newTitle: string
  /** 两边逐行比较没有任何差异时的提示文案 */
  identicalText: string
  className?: string
}

/** 斜线条纹类名：表示这一侧没有对应行（VS Code 填充格） */
const fillerStripe =
  'bg-[repeating-linear-gradient(-45deg,transparent,transparent_4px,rgb(0_0_0/0.04)_4px,rgb(0_0_0/0.04)_5px)] dark:bg-[repeating-linear-gradient(-45deg,transparent,transparent_4px,rgb(255_255_255/0.06)_4px,rgb(255_255_255/0.06)_5px)]'

/** 把分段列表渲染成带高亮底的文本节点（禁止 innerHTML） */
function renderSegments(segments: Segment[] | undefined, plain: string, side: 'old' | 'new'): JSX.Element | string {
  if (!segments || segments.length === 0) {
    return plain.length === 0 ? '\u00a0' : plain
  }
  const highlight =
    side === 'old'
      ? 'bg-red-500/25 dark:bg-red-400/30 rounded-sm'
      : 'bg-green-500/25 dark:bg-green-400/30 rounded-sm'
  return (
    <>
      {segments.map((seg, i) =>
        seg.changed ? (
          <span key={i} className={highlight}>
            {seg.text || '\u00a0'}
          </span>
        ) : (
          <span key={i}>{seg.text}</span>
        ),
      )}
    </>
  )
}

interface RowCellsProps {
  row: DiffRow
  contentRef?: RefObject<HTMLDivElement | null>
}

/**
 * 一行 diff 占 grid 同一行四格：左行号、左内容、右行号、右内容。
 * 折行时 CSS grid 自动让左右对齐，不需要 JS 同步滚动。
 */
function DiffRowCells({ row, contentRef }: RowCellsProps) {
  const { kind } = row

  const oldBg =
    kind === 'delete' || kind === 'change'
      ? 'bg-red-500/10 dark:bg-red-400/15'
      : kind === 'insert'
        ? fillerStripe
        : ''

  const newBg =
    kind === 'insert' || kind === 'change'
      ? 'bg-green-500/10 dark:bg-green-400/15'
      : kind === 'delete'
        ? fillerStripe
        : ''

  const oldMark = kind === 'delete' || kind === 'change' ? '−' : null
  const newMark = kind === 'insert' || kind === 'change' ? '+' : null

  const oldGutter = cn(
    'select-none text-right tabular-nums text-muted-foreground px-2 py-0.5 min-w-[3.25rem]',
    oldBg,
  )
  const oldContent = cn(
    'font-mono text-[length:var(--text-ui)] whitespace-pre-wrap break-words px-2 py-0.5 min-h-[1.25rem]',
    oldBg,
  )
  const newGutter = cn(
    'select-none text-right tabular-nums text-muted-foreground px-2 py-0.5 min-w-[3.25rem] border-border/40 border-l',
    newBg,
  )
  const newContent = cn(
    'font-mono text-[length:var(--text-ui)] whitespace-pre-wrap break-words px-2 py-0.5 min-h-[1.25rem]',
    newBg,
  )

  return (
    <>
      <div className={oldGutter}>
        {row.oldNo != null ? (
          <>
            {oldMark != null && (
              <span className="mr-1 text-red-600 dark:text-red-400" aria-hidden>
                {oldMark}
              </span>
            )}
            {row.oldNo}
          </>
        ) : null}
      </div>
      <div ref={contentRef} className={oldContent} data-kind={kind}>
        {kind === 'insert' ? null : renderSegments(row.oldSegments, row.oldText, 'old')}
      </div>
      <div className={newGutter}>
        {row.newNo != null ? (
          <>
            {newMark != null && (
              <span className="mr-1 text-green-600 dark:text-green-400" aria-hidden>
                {newMark}
              </span>
            )}
            {row.newNo}
          </>
        ) : null}
      </div>
      <div className={newContent} data-kind={kind}>
        {kind === 'delete' ? null : renderSegments(row.newSegments, row.newText, 'new')}
      </div>
    </>
  )
}

/**
 * 并排文本对比：布局仿 VS Code git diff，四列 grid + sticky 标题与增删统计。
 * 高度由外层决定：外层限制了最大高度时，只有行区域自己滚动、标题栏不动；外层不限制就按内容撑开。
 */
export function DiffView({
  oldText,
  newText,
  oldTitle,
  newTitle,
  identicalText,
  className,
}: DiffViewProps): JSX.Element {
  const rows = useMemo(() => diffLines(oldText, newText), [oldText, newText])
  const stats = useMemo(() => diffStats(rows), [rows])
  const hasDiff = rows.some((r) => r.kind !== 'equal')
  const scrollerRef = useRef<HTMLDivElement>(null)
  const firstChangeRef = useRef<HTMLDivElement>(null)

  // 打开或内容变化后，把第一处改动滚到对比区中间；只改对比区自己的 scrollTop，不影响外层弹窗和页面
  useEffect(() => {
    const scroller = scrollerRef.current
    const target = firstChangeRef.current
    if (!scroller || !target) return
    scroller.scrollTop = Math.max(0, target.offsetTop - scroller.clientHeight / 2 + target.offsetHeight / 2)
  }, [hasDiff, oldText, newText])

  if (!hasDiff) {
    return (
      <div className={cn('rounded-md border border-border px-4 py-6 text-center text-muted-foreground', className)}>
        {identicalText}
      </div>
    )
  }

  const firstChangeIndex = rows.findIndex((r) => r.kind !== 'equal')

  return (
    <div className={cn('flex min-h-0 flex-col overflow-hidden rounded-md border border-border', className)}>
      <div className="sticky top-0 z-10 grid grid-cols-2 border-b border-border bg-muted/80 backdrop-blur-sm text-[length:var(--text-ui)]">
        <div className="px-3 py-2 font-medium border-r border-border">{oldTitle}</div>
        <div className="px-3 py-2 font-medium flex items-center justify-between gap-2">
          <span>{newTitle}</span>
          <span className="text-muted-foreground tabular-nums font-normal">
            {stats.added > 0 && <span className="text-green-600 dark:text-green-400">+{stats.added}</span>}
            {stats.added > 0 && stats.removed > 0 && ' '}
            {stats.removed > 0 && <span className="text-red-600 dark:text-red-400">−{stats.removed}</span>}
          </span>
        </div>
      </div>
      <div ref={scrollerRef} className="relative min-h-0 flex-1 overflow-auto">
        <div className="grid min-w-full" style={{ gridTemplateColumns: 'auto minmax(0,1fr) auto minmax(0,1fr)' }}>
          {rows.map((row, i) => (
            <DiffRowCells
              key={`${row.kind}-${row.oldNo ?? 'n'}-${row.newNo ?? 'n'}-${i}`}
              row={row}
              contentRef={i === firstChangeIndex ? firstChangeRef : undefined}
            />
          ))}
        </div>
      </div>
    </div>
  )
}