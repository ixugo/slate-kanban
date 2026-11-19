import { readdirSync, readFileSync, statSync } from 'node:fs'
import path from 'node:path'
import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import i18n, { initialLanguage, loadLanguage, setLanguage } from './index'
import { LANG_STORAGE_KEY, LANGUAGES } from './languages'
import { en } from './locales/en'
import { ja } from './locales/ja'
import { ko } from './locales/ko'
import { zhCN } from './locales/zh-CN'
import { zhTW } from './locales/zh-TW'
import { useTranslation } from './react'
import { getByPath } from './lookup'

type Tree = { [key: string]: string | Tree }

// 把嵌套的文案对象拍平成 "a.b.c" 形式的键值表。
function flatten(tree: Tree, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(tree)) {
    const full = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') out[full] = value
    else Object.assign(out, flatten(value, full))
  }
  return out
}

// 取出文案里的全部占位符名称，排好序，方便比较。
function placeholders(text: string): string[] {
  return [...text.matchAll(/\{\{\s*(\w+)\s*\}\}/g)].map((m) => m[1]).sort()
}

const base = flatten(en)
const locales = [
  ['en', en],
  ['zh-CN', zhCN],
  ['zh-TW', zhTW],
  ['ja', ja],
  ['ko', ko],
] as const

describe('各语言的文案', () => {
  it.each(locales)('%s 的键和英文完全一致（运行期再核对一遍类型约束）', (_code, messages) => {
    expect(Object.keys(flatten(messages as Tree)).sort()).toEqual(Object.keys(base).sort())
  })

  it.each(locales)('%s 没有空文案', (_code, messages) => {
    for (const [key, text] of Object.entries(flatten(messages as Tree))) {
      expect(text.trim(), key).not.toBe('')
    }
  })

  it.each(locales)('%s 的占位符和英文一致，不会显示出没替换的 {{xxx}}', (_code, messages) => {
    const flat = flatten(messages as Tree)
    for (const [key, text] of Object.entries(flat)) {
      expect(placeholders(text), key).toEqual(placeholders(base[key]))
    }
  })

  it('五种语言都登记了资源', () => {
    expect(LANGUAGES.map((l) => l.code).sort()).toEqual(['en', 'ja', 'ko', 'zh-CN', 'zh-TW'])
  })
})

// 递归找出一个目录下所有的 .go 文件（不含测试文件）。
function goFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = path.join(dir, name)
    if (statSync(full).isDirectory()) return goFiles(full)
    return name.endsWith('.go') && !name.endsWith('_test.go') ? [full] : []
  })
}

describe('服务端错误的翻译', () => {
  const coreDir = path.resolve(import.meta.dirname, '../../../internal/core')
  const declared = goFiles(coreDir).flatMap((file) =>
    [...readFileSync(file, 'utf8').matchAll(/reason\.NewError\("(\w+)"/g)].map((m) => m[1]),
  )

  it('能从 Go 源码里找到错误定义（防止路径变了测试悄悄失效）', () => {
    expect(declared.length).toBeGreaterThan(20)
    expect(declared).toContain('ErrBoardNotFound')
    expect(declared).toContain('ErrCardVersionConflict')
  })

  it.each(declared)('领域错误 %s 有翻译，新增错误时这里会提醒补文案', (name) => {
    expect(Object.keys(en.errors), `${name} 缺少翻译`).toContain(name)
  })

  // 经过 REST 层或框架可能返回给网页的通用错误，以及网页自己产生的网络错误
  it.each([
    'ErrBadRequest',
    'ErrContentTooLarge',
    'ErrUnsupportedMediaType',
    'ErrTooManyRequests',
    'ErrInternal',
    'ErrStore',
    'ErrServer',
    'ErrServiceUnavailable',
    'ErrTimeout',
    'ErrNotFound',
    'ErrConflict',
    'ErrPermissionDenied',
    'ErrNetwork',
    'ErrBadResponse',
  ])('通用错误 %s 有翻译', (name) => {
    expect(Object.keys(en.errors)).toContain(name)
  })
})

describe('语言的选择和记忆', () => {
  it('没选过语言时跟随浏览器的偏好', () => {
    expect(initialLanguage()).toBe('en')
  })

  it('选过的语言存进 localStorage，下次启动优先使用，并且同步 <html lang>', async () => {
    await setLanguage('ja')
    expect(localStorage.getItem(LANG_STORAGE_KEY)).toBe('ja')
    expect(initialLanguage()).toBe('ja')
    expect(document.documentElement.lang).toBe('ja')
    expect(i18n.t('common.save')).toBe('保存')
    await setLanguage('zh-TW')
    expect(document.documentElement.lang).toBe('zh-TW')
    expect(i18n.t('common.save')).toBe('儲存')
    await setLanguage('en')
  })

  it('存储里是不认识的值时忽略它', () => {
    localStorage.setItem(LANG_STORAGE_KEY, 'xx')
    expect(initialLanguage()).toBe('en')
  })

  it('文案里的占位符能被替换', async () => {
    await setLanguage('ko')
    expect(i18n.t('boards.updated', { time: '3분 전' })).toBe('3분 전 업데이트')
    await setLanguage('en')
    expect(i18n.t('boards.updated', { time: '3 min ago' })).toBe('Updated 3 min ago')
  })

  it('缺译键回退英文，英文也没有时返回 key', async () => {
    await setLanguage('en')
    expect(getByPath(en, 'common.save')).toBe('Save')
    expect(i18n.t('nonexistent.key' as 'common.save')).toBe('nonexistent.key')
  })

  it('loadLanguage 之后可以正常取译文', async () => {
    await loadLanguage('ja')
    await setLanguage('ja')
    expect(i18n.t('nav.boards')).toBe(ja.nav.boards)
    await setLanguage('en')
  })

  it('语言切换触发订阅者重新渲染', async () => {
    const { result } = renderHook(() => useTranslation())
    expect(result.current.t('common.save')).toBe('Save')
    await act(async () => {
      await setLanguage('ja')
    })
    expect(result.current.t('common.save')).toBe('保存')
  })
})