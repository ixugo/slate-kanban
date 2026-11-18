import { readdirSync, readFileSync, statSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, it } from 'vitest'
import i18n, { initialLanguage, resources, setLanguage } from './index'
import { LANG_STORAGE_KEY, LANGUAGES } from './languages'
import { en } from './locales/en'

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
const locales = Object.entries(resources).map(([code, r]) => [code, flatten(r.translation as Tree)] as const)

describe('各语言的文案', () => {
  it.each(locales)('%s 的键和英文完全一致（运行期再核对一遍类型约束）', (_code, messages) => {
    expect(Object.keys(messages).sort()).toEqual(Object.keys(base).sort())
  })

  it.each(locales)('%s 没有空文案', (_code, messages) => {
    for (const [key, text] of Object.entries(messages)) {
      expect(text.trim(), key).not.toBe('')
    }
  })

  it.each(locales)('%s 的占位符和英文一致，不会显示出没替换的 {{xxx}}', (_code, messages) => {
    for (const [key, text] of Object.entries(messages)) {
      expect(placeholders(text), key).toEqual(placeholders(base[key]))
    }
  })

  it('五种语言都登记了资源', () => {
    expect(Object.keys(resources).sort()).toEqual(LANGUAGES.map((l) => l.code).sort())
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
    expect(declared).toContain('ErrTokenNameTaken')
  })

  it.each(declared)('领域错误 %s 有翻译，新增错误时这里会提醒补文案', (name) => {
    expect(Object.keys(en.errors), `${name} 缺少翻译`).toContain(name)
  })

  // 经过 REST 层或框架可能返回给网页的通用错误，以及网页自己产生的网络错误
  it.each([
    'ErrBadRequest',
    'ErrUnauthorized',
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
    expect(i18n.t('login.title')).toBe('ログイン')
    await setLanguage('zh-TW')
    expect(document.documentElement.lang).toBe('zh-TW')
    expect(i18n.t('login.title')).toBe('登入')
    await setLanguage('en')
  })

  it('存储里是不认识的值时忽略它', () => {
    localStorage.setItem(LANG_STORAGE_KEY, 'xx')
    expect(initialLanguage()).toBe('en')
  })

  it('文案里的占位符能被替换', async () => {
    await setLanguage('ko')
    expect(i18n.t('account.signedInAs', { name: 'admin' })).toBe('admin(으)로 로그인됨')
    await setLanguage('en')
    expect(i18n.t('account.signedInAs', { name: 'admin' })).toBe('Signed in as admin')
  })
})
