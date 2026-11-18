import { describe, expect, it } from 'vitest'
import { coverSrc } from './cover'

const A = '/api/v1/attachments/' + 'a'.repeat(32)
const B = '/api/v1/attachments/' + 'b'.repeat(32)

describe('coverSrc', () => {
  it('取描述里的第一张图片附件', () => {
    expect(coverSrc(`文字\n\n![x](${A})\n\n![y](${B})`)).toBe(A)
  })

  it('没有图片、空描述返回空串', () => {
    expect(coverSrc('')).toBe('')
    expect(coverSrc('没有图')).toBe('')
  })

  it('外部图片地址和非法附件地址不算封面，继续找后面的', () => {
    expect(coverSrc(`![e](https://a.com/x.png)\n![bad](/api/v1/attachments/xyz)\n![ok](${B})`)).toBe(B)
    expect(coverSrc('![e](https://a.com/x.png)')).toBe('')
  })

  it('代码块里的图片语法不算', () => {
    expect(coverSrc(`\`\`\`md\n![x](${A})\n\`\`\`\n\n![y](${B})`)).toBe(B)
    expect(coverSrc(`\`\`\`\n![x](${A})`)).toBe('')
  })
})
