import { describe, expect, it } from 'vitest'
import i18n from '@/i18n'
import { en } from '@/i18n/locales/en'
import { ja } from '@/i18n/locales/ja'
import { ApiError, REASON_NETWORK } from '@/lib/api/client'
import { describeError, isAbortError, isVersionConflict } from './errors'

const tEn = i18n.getFixedT('en')
const tJa = i18n.getFixedT('ja')

describe('describeError', () => {
  it('有翻译的错误按当前语言显示，不显示服务端的中文文案', () => {
    const err = new ApiError(404, 'ErrBoardNotFound', '看板不存在', 'trace-1')
    expect(describeError(err, tEn)).toEqual({ message: en.errors.ErrBoardNotFound, detail: '', traceId: 'trace-1' })
    expect(describeError(err, tJa).message).toBe(ja.errors.ErrBoardNotFound)
    expect(en.errors.ErrBoardNotFound).not.toBe(ja.errors.ErrBoardNotFound)
  })

  it('没有翻译的错误退回服务端文案', () => {
    const err = new ApiError(418, 'ErrTeapot', '我是茶壶')
    expect(describeError(err, tEn)).toEqual({ message: '我是茶壶', detail: '', traceId: '' })
  })

  it('没有翻译、服务端文案也是空的，显示通用提示', () => {
    expect(describeError(new ApiError(500, 'Http500', ''), tEn).message).toBe(en.errors.title)
  })

  it('校验类错误把服务端写明的具体原因放进 detail', () => {
    const err = new ApiError(400, 'ErrInvalidInput', '标题不能为空')
    const text = describeError(err, tEn)
    expect(text.message).toBe(en.errors.ErrInvalidInput)
    expect(text.detail).toBe('标题不能为空')
  })

  it('非校验类错误不带 detail', () => {
    const err = new ApiError(404, 'ErrBoardNotFound', '看板不存在')
    expect(describeError(err, tEn).detail).toBe('')
  })

  it('网络错误有翻译', () => {
    expect(describeError(new ApiError(0, REASON_NETWORK, 'Failed to fetch'), tEn).message).toBe(en.errors.ErrNetwork)
  })

  it('错误名叫 title 或 traceId 时不会误取到非错误文案', () => {
    expect(describeError(new ApiError(500, 'title', 'x'), tEn).message).toBe('x')
    expect(describeError(new ApiError(500, 'traceId', 'y'), tEn).message).toBe('y')
  })

  it('错误名是 Object 原型上的属性名时也不会误判', () => {
    expect(describeError(new ApiError(500, 'toString', 'z'), tEn).message).toBe('z')
  })

  it('普通 Error 显示它的 message，其他值显示通用提示', () => {
    expect(describeError(new Error('boom'), tEn).message).toBe('boom')
    expect(describeError('weird', tEn).message).toBe(en.errors.title)
    expect(describeError(null, tEn).message).toBe(en.errors.title)
  })
})

describe('isAbortError / isVersionConflict', () => {
  it('只认 AbortError', () => {
    expect(isAbortError(new DOMException('x', 'AbortError'))).toBe(true)
    expect(isAbortError(new DOMException('x', 'NetworkError'))).toBe(false)
    expect(isAbortError(new Error('AbortError'))).toBe(false)
  })

  it('只认版本冲突的错误名', () => {
    expect(isVersionConflict(new ApiError(409, 'ErrCardVersionConflict', ''))).toBe(true)
    expect(isVersionConflict(new ApiError(409, 'ErrConflict', ''))).toBe(false)
    expect(isVersionConflict(new Error('x'))).toBe(false)
  })
})
