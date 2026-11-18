// 卡片封面：取描述里出现的第一张图片附件，看板磁贴和卡片弹窗顶部都用它。

import { isImageSrc } from './inline'

/** 代码块里的图片语法只是示例文字，不能当封面 */
const FENCED_CODE = /^(```|~~~)[^\n]*\n[\s\S]*?(?:\n\1[^\n]*(?=\n|$)|$)/gm
/** Markdown 图片：![说明](地址)，地址是否合法交给 isImageSrc 判断 */
const IMAGE_SYNTAX = /!\[[^\]\n]*\]\(([^)\s]+)\)/g

/** 返回描述里第一张图片附件的地址；没有就返回空串 */
export function coverSrc(description: string): string {
  if (!description.includes('![')) return ''
  const text = description.replace(FENCED_CODE, '')
  for (const match of text.matchAll(IMAGE_SYNTAX)) {
    if (isImageSrc(match[1])) return match[1]
  }
  return ''
}
