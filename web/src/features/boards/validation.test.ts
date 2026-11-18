import { describe, expect, it } from 'vitest'
import { isAbsolutePath, validateBoardForm } from './validation'

const ok = { name: 'Slate', slug: '', projectPath: '' }

describe('isAbsolutePath', () => {
  it.each(['/code/app', '/', '\\\\server\\share', 'C:\\code', 'c:/code', 'Z:\\'])('%s 是绝对路径', (p) => {
    expect(isAbsolutePath(p)).toBe(true)
  })

  it.each(['code/app', './app', '~/app', 'C:app', '1:/app', '', 'C:'])('%s 不是绝对路径', (p) => {
    expect(isAbsolutePath(p)).toBe(false)
  })
})

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

  it('项目路径：可以留空；必须是绝对路径，不能有控制字符，不能超过 512 个字符', () => {
    expect(validateBoardForm({ ...ok, projectPath: '/code/app' })).toEqual({})
    expect(validateBoardForm({ ...ok, projectPath: 'C:\\code\\app' })).toEqual({})
    expect(validateBoardForm({ ...ok, projectPath: 'relative/path' }).projectPath).toBe('pathInvalid')
    expect(validateBoardForm({ ...ok, projectPath: '/a\nb' }).projectPath).toBe('pathInvalid')
    expect(validateBoardForm({ ...ok, projectPath: '/a\x00b' }).projectPath).toBe('pathInvalid')
    expect(validateBoardForm({ ...ok, projectPath: '/a\x7fb' }).projectPath).toBe('pathInvalid')
    expect(validateBoardForm({ ...ok, projectPath: '/a\x85b' }).projectPath).toBe('pathInvalid')
    expect(validateBoardForm({ ...ok, projectPath: '/' + '路'.repeat(511) })).toEqual({})
    expect(validateBoardForm({ ...ok, projectPath: '/' + '😀'.repeat(512) }).projectPath).toBe('pathInvalid')
    expect(validateBoardForm({ ...ok, projectPath: '/' + 'a'.repeat(511) })).toEqual({})
    expect(validateBoardForm({ ...ok, projectPath: '/' + 'a'.repeat(512) }).projectPath).toBe('pathInvalid')
  })

  it('多个字段同时出错时全部返回', () => {
    expect(validateBoardForm({ name: '', slug: '1', projectPath: 'x' })).toEqual({
      name: 'nameRequired',
      slug: 'slugInvalid',
      projectPath: 'pathInvalid',
    })
  })
})
