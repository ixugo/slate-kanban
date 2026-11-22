import { describe, expect, it } from 'vitest'
import { firstImageSrc } from './cover'

const A = '/api/v1/attachments/' + 'a'.repeat(32)
const B = '/api/v1/attachments/' + 'b'.repeat(32)

describe('firstImageSrc', () => {
  it('没有图片、空描述、只有外链图片都返回空', () => {
    expect(firstImageSrc('')).toBe('')
    expect(firstImageSrc('只有文字')).toBe('')
    expect(firstImageSrc('![x](https://example.com/a.png)')).toBe('')
  })

  it('取按阅读顺序的第一张本服务图片附件', () => {
    expect(firstImageSrc(`说明\n\n![一](${A})\n\n![二](${B})`)).toBe(A)
  })

  it('列表、引用、标题、粗体里的图片也算', () => {
    expect(firstImageSrc(`- 项\n  - ![深](${A})`)).toBe(A)
    expect(firstImageSrc(`> ![引](${A})`)).toBe(A)
    expect(firstImageSrc(`# ![标题](${A})`)).toBe(A)
    expect(firstImageSrc(`**![粗](${A})**`)).toBe(A)
  })

  it('代码块和行内代码里写的图片语法不算', () => {
    expect(firstImageSrc('```\n![x](' + A + ')\n```')).toBe('')
    expect(firstImageSrc('`![x](' + A + ')`')).toBe('')
    expect(firstImageSrc('```\n![x](' + A + ')\n```\n\n![y](' + B + ')')).toBe(B)
  })
})
