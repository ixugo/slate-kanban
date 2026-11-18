import { describe, expect, it } from 'vitest'
import { LABEL_HUES, LABEL_SHADES, labelStyle, parseLabelColor } from './labelColors'

describe('labelColors', () => {
  it('30 种颜色都能解析并有样式，浅色档用深字，其余用白字', () => {
    let count = 0
    for (const hue of LABEL_HUES) {
      for (const shade of LABEL_SHADES) {
        const style = labelStyle(`${hue}-${shade}`)
        expect(style?.backgroundColor).toMatch(/^#[0-9a-f]{6}$/)
        expect(style?.color).toBe(shade === 3 ? '#1d2125' : '#ffffff')
        count++
      }
    }
    expect(count).toBe(30)
  })

  it('空串和不认识的编号没有样式', () => {
    for (const bad of ['', 'teal-2', 'red-4', 'red', 'red-0', '-1']) {
      expect(parseLabelColor(bad)).toBeNull()
      expect(labelStyle(bad)).toBeUndefined()
    }
  })
})
