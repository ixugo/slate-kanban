// 任务封面：取描述里按阅读顺序出现的第一张本服务图片附件，看板磁贴把它放在顶部。
// 复用块级、行内解析器，所以代码块、行内代码里写的图片语法不会被当成封面。

import { parseBlocks, type Block } from './blocks'
import { parseInline, type Inline } from './inline'

/** 描述里没有附件地址时一定没有封面，先用它跳过解析，避免给每张磁贴都跑一遍解析器 */
const ATTACHMENT_HINT = '/api/v1/attachments/'

/** 在一段行内节点里按顺序找第一张图片的地址；没有返回空串 */
function imageInInline(nodes: readonly Inline[]): string {
  for (const node of nodes) {
    if (node.kind === 'image') return node.src
    if (node.kind === 'span' || node.kind === 'link') {
      const found = imageInInline(node.children)
      if (found) return found
    }
  }
  return ''
}

/** 在块列表里按顺序找第一张图片的地址；代码块不看，引用和列表递归进去 */
function imageInBlocks(blocks: readonly Block[]): string {
  for (const block of blocks) {
    let found = ''
    if (block.kind === 'p' || block.kind === 'heading') {
      found = imageInInline(parseInline(block.text))
    } else if (block.kind === 'quote') {
      found = imageInBlocks(block.children)
    } else if (block.kind === 'list') {
      for (const item of block.items) {
        found = imageInInline(parseInline(item.text)) || imageInBlocks(item.children)
        if (found) break
      }
    }
    if (found) return found
  }
  return ''
}

/** 返回描述里的第一张图片地址，没有就返回空串 */
export function firstImageSrc(description: string): string {
  if (!description.includes(ATTACHMENT_HINT)) return ''
  return imageInBlocks(parseBlocks(description))
}
