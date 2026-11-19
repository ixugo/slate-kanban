import { describe, expect, it } from 'vitest'
import { validateBoardForm } from './validation'

const ok = { name: 'Slate', slug: '' }

describe('validateBoardForm', () => {
  it('最小的合法表单：只有名称', () => {
    expect(validateBoardForm(ok)).toEqual({})
  })

  it('名称：空白或只有空格不行，100 个字符行，101 个不行（按字符数，不是字节数）', () => {
    expect(validateBoardForm({ ...ok, name: '' }).name).toBe('nameRequired')
    expect(validateBoardForm({ ...ok, name: '   ' }).name).toBe('nameRequired')
    expect(validateBoardForm({ ...ok, name: 'a'.repeat(100) })).toEqual({})
    expect(validateBoardForm({ ...ok, name: 'a'.repeat(101) }).name).toBe('nameRequired')
    expect(validateBoardForm({ ...ok, name: '看'.repeat(100) })).toEqual({})
    expect(validateBoardForm({ ...ok, name: '😀'.repeat(100) })).toEqual({})
    expect(validateBoardForm({ ...ok, name: '😀'.repeat(101) }).name).toBe('nameRequired')
  })

  it('编号前缀：可以留空；小写会按大写判断；2 到 10 位，字母开头', () => {
    expect(validateBoardForm({ ...ok, slug: '' })).toEqual({})
    expect(validateBoardForm({ ...ok, slug: 'slate' })).toEqual({})
    expect(validateBoardForm({ ...ok, slug: 'AB' })).toEqual({})
    expect(validateBoardForm({ ...ok, slug: 'A1B2C3D4E5' })).toEqual({})
    for (const bad of ['A', 'A1B2C3D4E5F', '1ABC', 'AB-C', 'AB C', '编号']) {
      expect(validateBoardForm({ ...ok, slug: bad }).slug, bad).toBe('slugInvalid')
    }
  })

  it('多个字段同时出错时全部返回', () => {
    expect(validateBoardForm({ name: '', slug: '1' })).toEqual({
      name: 'nameRequired',
      slug: 'slugInvalid',
    })
  })
})
