// 安全的 Markdown 渲染：不解析原始 HTML，链接只放行 http、https、mailto。
// 不接 react-markdown，避免把解析库打进首屏；输出全是 React 节点，不用 dangerouslySetInnerHTML。

import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

/** 判断链接协议是否允许在新标签页打开 */
function isSafeHref(href: string): boolean {
  const trimmed = href.trim()
  return /^(https?:\/\/|mailto:)/i.test(trimmed)
}

/** 把行内 Markdown（粗体、斜体、代码、链接）拆成 React 片段 */
function parseInline(text: string, keyPrefix: string): ReactNode[] {
  const out: ReactNode[] = []
  let rest = text
  let part = 0
  const pushText = (s: string) => {
    if (s) out.push(s)
  }
  while (rest.length > 0) {
    const code = rest.match(/^`([^`]+)`/)
    if (code) {
      pushText('')
      out.push(
        <code key={`${keyPrefix}-c${part++}`} className="rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[0.9em]">
          {code[1]}
        </code>,
      )
      rest = rest.slice(code[0].length)
      continue
    }
    const link = rest.match(/^\[([^\]]+)\]\(([^)]+)\)/)
    if (link) {
      const label = link[1]
      const href = link[2]
      if (isSafeHref(href)) {
        out.push(
          <a
            key={`${keyPrefix}-a${part++}`}
            href={href}
            rel="noopener noreferrer"
            target="_blank"
            className="text-primary underline underline-offset-2"
          >
            {label}
          </a>,
        )
      } else {
        pushText(`[${label}](${href})`)
      }
      rest = rest.slice(link[0].length)
      continue
    }
    const bold = rest.match(/^\*\*([^*]+)\*\*/)
    if (bold) {
      out.push(<strong key={`${keyPrefix}-b${part++}`}>{bold[1]}</strong>)
      rest = rest.slice(bold[0].length)
      continue
    }
    const italic = rest.match(/^\*([^*]+)\*/)
    if (italic) {
      out.push(<em key={`${keyPrefix}-i${part++}`}>{italic[1]}</em>)
      rest = rest.slice(italic[0].length)
      continue
    }
    const next = rest.search(/[`[*]/)
    if (next === -1) {
      pushText(rest)
      break
    }
    pushText(rest.slice(0, next))
    rest = rest.slice(next)
  }
  return out.length > 0 ? out : [text]
}

type Block =
  | { kind: 'code'; lines: string[] }
  | { kind: 'heading'; level: number; text: string }
  | { kind: 'hr' }
  | { kind: 'quote'; lines: string[] }
  | { kind: 'ul'; items: { checked?: boolean; text: string }[] }
  | { kind: 'ol'; items: string[] }
  | { kind: 'p'; text: string }

/** 把全文切成块级结构，再交给 React 渲染 */
function parseBlocks(source: string): Block[] {
  const lines = source.replace(/\r\n/g, '\n').split('\n')
  const blocks: Block[] = []
  let i = 0
  while (i < lines.length) {
    const line = lines[i]
    if (line.trim() === '') {
      i += 1
      continue
    }
    if (line.startsWith('```')) {
      const buf: string[] = []
      i += 1
      while (i < lines.length && !lines[i].startsWith('```')) {
        buf.push(lines[i])
        i += 1
      }
      if (i < lines.length) i += 1
      blocks.push({ kind: 'code', lines: buf })
      continue
    }
    const heading = line.match(/^(#{1,3})\s+(.+)$/)
    if (heading) {
      blocks.push({ kind: 'heading', level: heading[1].length, text: heading[2] })
      i += 1
      continue
    }
    if (/^(-{3,}|\*{3,}|_{3,})\s*$/.test(line.trim())) {
      blocks.push({ kind: 'hr' })
      i += 1
      continue
    }
    if (line.startsWith('>')) {
      const buf: string[] = []
      while (i < lines.length && (lines[i].startsWith('>') || lines[i].trim() === '')) {
        if (lines[i].startsWith('>')) buf.push(lines[i].replace(/^>\s?/, ''))
        else if (buf.length > 0) break
        i += 1
      }
      blocks.push({ kind: 'quote', lines: buf })
      continue
    }
    const task = line.match(/^[-*]\s+\[([ xX])\]\s+(.+)$/)
    const bullet = line.match(/^[-*]\s+(.+)$/)
    const ordered = line.match(/^\d+\.\s+(.+)$/)
    if (task || bullet) {
      const items: { checked?: boolean; text: string }[] = []
      while (i < lines.length) {
        const t = lines[i].match(/^[-*]\s+\[([ xX])\]\s+(.+)$/)
        const b = lines[i].match(/^[-*]\s+(.+)$/)
        if (t) items.push({ checked: t[1].toLowerCase() === 'x', text: t[2] })
        else if (b && !lines[i].match(/^[-*]\s+\[/)) items.push({ text: b[1] })
        else break
        i += 1
      }
      blocks.push({ kind: 'ul', items })
      continue
    }
    if (ordered) {
      const items: string[] = []
      while (i < lines.length) {
        const m = lines[i].match(/^\d+\.\s+(.+)$/)
        if (!m) break
        items.push(m[1])
        i += 1
      }
      blocks.push({ kind: 'ol', items })
      continue
    }
    const para: string[] = [line]
    i += 1
    while (i < lines.length && lines[i].trim() !== '' && !/^#{1,3}\s/.test(lines[i]) && !lines[i].startsWith('```')) {
      para.push(lines[i])
      i += 1
    }
    blocks.push({ kind: 'p', text: para.join('\n') })
  }
  return blocks
}

/** 渲染一整段 Markdown 文本 */
export function Markdown({ source, className }: { source: string; className?: string }) {
  const blocks = parseBlocks(source)
  return (
    <div className={cn('markdown space-y-3 text-ui leading-relaxed break-words', className)}>
      {blocks.map((block, idx) => {
        const key = `md-${idx}`
        if (block.kind === 'code') {
          return (
            <pre key={key} className="overflow-x-auto rounded-xl bg-secondary/70 p-3.5 font-mono text-xs">
              <code>{block.lines.join('\n')}</code>
            </pre>
          )
        }
        if (block.kind === 'heading') {
          const Tag = block.level === 1 ? 'h1' : block.level === 2 ? 'h2' : 'h3'
          const cls =
            block.level === 1 ? 'text-lg font-semibold' : block.level === 2 ? 'text-base font-semibold' : 'text-sm font-semibold'
          return (
            <Tag key={key} className={cls}>
              {parseInline(block.text, key)}
            </Tag>
          )
        }
        if (block.kind === 'hr') return <hr key={key} className="border-border" />
        if (block.kind === 'quote') {
          return (
            <blockquote key={key} className="border-l-2 border-muted-foreground/40 pl-3 text-muted-foreground">
              {block.lines.map((ln, j) => (
                <p key={j}>{parseInline(ln, `${key}-q${j}`)}</p>
              ))}
            </blockquote>
          )
        }
        if (block.kind === 'ul') {
          return (
            <ul key={key} className="list-none space-y-1 pl-0">
              {block.items.map((item, j) => (
                <li key={j} className="flex gap-2">
                  {item.checked !== undefined ? (
                    <span aria-hidden className="mt-0.5 font-mono text-muted-foreground">
                      {item.checked ? '☑' : '☐'}
                    </span>
                  ) : (
                    <span aria-hidden className="text-muted-foreground">
                      •
                    </span>
                  )}
                  <span>{parseInline(item.text, `${key}-u${j}`)}</span>
                </li>
              ))}
            </ul>
          )
        }
        if (block.kind === 'ol') {
          return (
            <ol key={key} className="list-decimal space-y-1 pl-5">
              {block.items.map((item, j) => (
                <li key={j}>{parseInline(item, `${key}-o${j}`)}</li>
              ))}
            </ol>
          )
        }
        return (
          <p key={key} className="whitespace-pre-wrap">
            {parseInline(block.text, key)}
          </p>
        )
      })}
    </div>
  )
}