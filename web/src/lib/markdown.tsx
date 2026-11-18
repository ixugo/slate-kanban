// 安全的 Markdown 渲染：不解析原始 HTML，链接只放行 http、https、mailto，图片只显示本服务的附件。
// 输出全是 React 节点，不用 dangerouslySetInnerHTML；也不接 react-markdown，避免把解析库打进首屏。
// 解析交给 lib/md（和编辑器共用一份），这里只负责把块树和行内节点画成界面。

import { useId, type ReactNode } from 'react'
import { parseBlocks, type Block, type ListItem } from '@/lib/md/blocks'
import { parseInline, type Inline } from '@/lib/md/inline'
import { cn } from '@/lib/utils'

/** 各级标题的字号字重（下标就是级别，0 不用） */
const HEADING_CLASS = [
  '',
  'text-lg font-semibold',
  'text-base font-semibold',
  'text-sm font-semibold',
  'text-sm font-medium',
  'text-xs font-semibold',
  'text-xs font-medium text-muted-foreground',
] as const

const CODE_CLASS = 'rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[0.9em]'

/** 渲染时要用到的上下文：id 前缀用来给任务项的文字起不重复的 id，onToggleTask 不为空时勾选框可点 */
interface Ctx {
  prefix: string
  onToggleTask?: (line: number) => void
}

/** 把行内节点画成 React 片段；链接的协议已经在解析时筛过，这里只管画 */
function renderInline(nodes: readonly Inline[], key: string): ReactNode[] {
  return nodes.map((node, i) => {
    const k = `${key}.${i}`
    switch (node.kind) {
      case 'text':
        return node.text
      case 'esc':
        return node.char
      case 'code':
        return (
          <code key={k} className={CODE_CLASS}>
            {node.text}
          </code>
        )
      case 'image':
        return (
          <a key={k} href={node.src} target="_blank" rel="noopener noreferrer" className="inline-block max-w-full align-bottom">
            <img src={node.src} alt={node.alt} loading="lazy" className="max-h-96 max-w-full rounded-lg border border-border object-contain" />
          </a>
        )
      case 'link':
        return (
          <a key={k} href={node.href} rel="noopener noreferrer" target="_blank" className="text-primary underline underline-offset-2">
            {renderInline(node.children, k)}
          </a>
        )
      case 'span': {
        const Tag = node.tag
        return <Tag key={k}>{renderInline(node.children, k)}</Tag>
      }
    }
  })
}

/** 画一个列表项：任务项带勾选框（只读时禁用），后面跟文字和嵌套的子列表 */
function renderItem(item: ListItem, key: string, ctx: Ctx): ReactNode {
  const body = (
    <>
      {renderInline(parseInline(item.text), key)}
      {item.children.length > 0 && <div className="mt-1 space-y-1">{renderBlocks(item.children, `${key}c`, ctx)}</div>}
    </>
  )
  if (item.checked === undefined) return <li key={key}>{body}</li>
  const textId = `${ctx.prefix}${key}`
  return (
    <li key={key} className="-ml-5 flex list-none items-start gap-2">
      <input
        type="checkbox"
        className="mt-[0.35em] size-3.5 shrink-0 accent-primary"
        checked={item.checked}
        disabled={!ctx.onToggleTask}
        aria-labelledby={textId}
        onChange={() => ctx.onToggleTask?.(item.line)}
      />
      <div id={textId} className={cn('min-w-0 flex-1', item.checked && 'text-muted-foreground line-through')}>
        {body}
      </div>
    </li>
  )
}

/** 画一个块 */
function renderBlock(block: Block, key: string, ctx: Ctx): ReactNode {
  switch (block.kind) {
    case 'code':
      return (
        <pre key={key} className="overflow-x-auto rounded-xl bg-secondary/70 p-3.5 font-mono text-xs">
          <code>{block.text}</code>
        </pre>
      )
    case 'heading': {
      const Tag = `h${block.level}` as 'h1'
      return (
        <Tag key={key} className={HEADING_CLASS[block.level]}>
          {renderInline(parseInline(block.text), key)}
        </Tag>
      )
    }
    case 'hr':
      return <hr key={key} className="border-border" />
    case 'quote':
      return (
        <blockquote key={key} className="space-y-2 border-l-2 border-muted-foreground/40 pl-3 text-muted-foreground">
          {renderBlocks(block.children, key, ctx)}
        </blockquote>
      )
    case 'list': {
      const Tag = block.ordered ? 'ol' : 'ul'
      return (
        <Tag
          key={key}
          start={block.ordered && block.start !== 1 ? block.start : undefined}
          className={cn('space-y-1 pl-5', block.ordered ? 'list-decimal' : 'list-disc')}
        >
          {block.items.map((item, i) => renderItem(item, `${key}.${i}`, ctx))}
        </Tag>
      )
    }
    case 'p':
      return (
        <p key={key} className="whitespace-pre-wrap">
          {renderInline(parseInline(block.text), key)}
        </p>
      )
  }
}

/** 画一串块 */
function renderBlocks(blocks: readonly Block[], key: string, ctx: Ctx): ReactNode[] {
  return blocks.map((block, i) => renderBlock(block, `${key}${i}`, ctx))
}

/**
 * 渲染一整段 Markdown。
 * 传入 onToggleTask 时任务项的勾选框可以点，参数是它在原文里的行号（从 0 数），配合 toggleTaskAt 改写原文。
 */
export function Markdown({ source, className, onToggleTask }: { source: string; className?: string; onToggleTask?: (line: number) => void }) {
  const prefix = useId()
  return (
    <div className={cn('markdown space-y-3 text-ui leading-relaxed break-words', className)}>
      {renderBlocks(parseBlocks(source), 'md', { prefix, onToggleTask })}
    </div>
  )
}
