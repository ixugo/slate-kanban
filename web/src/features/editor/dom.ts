// 编辑器 DOM 和原文位置的互相换算。
// 渲染时根节点下每个 div 是一行，行内所有文字（含被 CSS 藏起来的记号）拼起来就是原文那一行；
// 行与行之间的 \n 不在 DOM 里，换算时按一个字符算。

/** 根节点下的所有行 */
export function lineEls(root: HTMLElement): HTMLElement[] {
  return Array.from(root.children) as HTMLElement[]
}

interface DomLine {
  nodes: ChildNode[]
  text: string
}

/** WebKit 上屏时可能把空行的 div 换成根节点下的文字和 BR。它们仍然是同一行。 */
function readLines(root: HTMLElement): DomLine[] {
  const lines: DomLine[] = []
  let inline: DomLine | null = null
  for (const node of root.childNodes) {
    if (node.nodeName === 'DIV' || node.nodeName === 'P') {
      if (inline) lines.push(inline)
      lines.push({ nodes: [node], text: node.textContent ?? '' })
      inline = null
    } else {
      inline ??= { nodes: [], text: '' }
      inline.nodes.push(node)
      if (node.nodeType === Node.TEXT_NODE || node.nodeType === Node.ELEMENT_NODE) inline.text += node.textContent ?? ''
      if (node.nodeName === 'BR') {
        lines.push(inline)
        inline = null
      }
    }
  }
  if (inline) lines.push(inline)
  return lines
}

/** 从 DOM 读出全文：每行的文字用 \n 连起来 */
export function readValue(root: HTMLElement): string {
  return readLines(root).map((line) => line.text).join('\n')
}

/** DOM 里的一个点（节点 + 偏移）换算成全文位置；点不在编辑器里返回 -1 */
export function offsetOf(root: HTMLElement, node: Node, offset: number): number {
  return offsetInLines(root, readLines(root), node, offset)
}

function offsetInLines(root: HTMLElement, lines: readonly DomLine[], node: Node, offset: number): number {
  let child: Node | null = node
  while (child && child !== root && child.parentNode !== root) child = child.parentNode
  if (!child) return -1

  let childEnd = 0
  let start = 0
  for (const line of lines) {
    childEnd += line.nodes.length
    if (child === root ? offset < childEnd : line.nodes.includes(child as ChildNode)) {
      const range = root.ownerDocument.createRange()
      range.setStartBefore(line.nodes[0])
      range.setEnd(node, offset)
      return start + range.toString().length
    }
    start += line.text.length + 1
  }
  return child === root ? Math.max(0, start - 1) : -1
}

/** DOM 里的一个点 */
export interface Point {
  node: Node
  offset: number
}

/**
 * 全文位置换算成 DOM 里的点。
 * 落在两段文字交界处时取后一段的开头（后一段才是看得见的）；
 * 整行都看不见的行（空行、只有被藏起来的记号）以末尾的 <br> 为准，光标停在 <br> 之前。
 */
export function pointOf(root: HTMLElement, pos: number): Point | null {
  const els = lineEls(root)
  if (els.length === 0) return null
  let at = 0
  let k = 0
  while (k < els.length - 1 && pos > at + (els[k].textContent ?? '').length) {
    at += (els[k].textContent ?? '').length + 1
    k++
  }
  const el = els[k]
  const col = Math.max(0, pos - at)
  if (el.lastChild?.nodeName === 'BR') return { node: el, offset: el.childNodes.length - 1 }
  const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT)
  let seen = 0
  let last: Text | null = null
  for (let t = walker.nextNode() as Text | null; t; t = walker.nextNode() as Text | null) {
    if (col < seen + t.length) return { node: t, offset: col - seen }
    seen += t.length
    last = t
  }
  return last ? { node: last, offset: last.length } : { node: el, offset: 0 }
}

/** 取当前选区在编辑器里的两端（锚点、焦点）；选区不在编辑器里返回 null */
export function readSelection(root: HTMLElement): { anchor: number; head: number } | null {
  const sel = root.ownerDocument.getSelection()
  if (!sel || !sel.anchorNode || !sel.focusNode) return null
  if (!root.contains(sel.anchorNode) || !root.contains(sel.focusNode)) return null
  const lines = readLines(root)
  const anchor = offsetInLines(root, lines, sel.anchorNode, sel.anchorOffset)
  const head = offsetInLines(root, lines, sel.focusNode, sel.focusOffset)
  return anchor < 0 || head < 0 ? null : { anchor, head }
}

/** 把选区设到 DOM 里；已经一样就不动，免得无谓地触发 selectionchange */
export function writeSelection(root: HTMLElement, anchor: number, head: number): void {
  const sel = root.ownerDocument.getSelection()
  const a = pointOf(root, anchor)
  const h = pointOf(root, head)
  if (!sel || !a || !h) return
  const same =
    sel.anchorNode === a.node && sel.anchorOffset === a.offset && sel.focusNode === h.node && sel.focusOffset === h.offset
  if (!same) sel.setBaseAndExtent(a.node, a.offset, h.node, h.offset)
}

/**
 * 把根节点下的行更新成 next。prev 是上一次画上去的 HTML：
 * 头尾没变的行原样保留（输入法组合、光标都不受影响），只替换中间变过的那一段；
 * prev 和实际行数对不上说明 DOM 被浏览器改过，整个重建。
 */
export function patchLines(root: HTMLElement, prev: readonly string[], next: readonly string[]): void {
  if (prev.length === 0 || prev.length !== root.children.length) {
    root.innerHTML = next.join('')
    return
  }
  let head = 0
  const max = Math.min(prev.length, next.length)
  while (head < max && prev[head] === next[head]) head++
  let tail = 0
  while (tail < max - head && prev[prev.length - 1 - tail] === next[next.length - 1 - tail]) tail++
  if (head === prev.length && head === next.length) return
  const els = lineEls(root)
  for (let i = head; i < prev.length - tail; i++) els[i].remove()
  const tpl = document.createElement('template')
  tpl.innerHTML = next.slice(head, next.length - tail).join('')
  root.insertBefore(tpl.content, root.children[head] ?? null)
}
