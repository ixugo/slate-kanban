import { describe, expect, it } from 'vitest'
import { LANGUAGES, normalizeLanguage, pickLanguage } from './languages'

describe('normalizeLanguage', () => {
  it.each([
    ['zh-TW', 'zh-TW'],
    ['zh_tw', 'zh-TW'],
    ['ZH-HK', 'zh-TW'],
    ['zh-MO', 'zh-TW'],
    ['zh-Hant', 'zh-TW'],
    ['zh-Hant-TW', 'zh-TW'],
    ['zh', 'zh-CN'],
    ['zh-CN', 'zh-CN'],
    ['zh-Hans', 'zh-CN'],
    ['zh-SG', 'zh-CN'],
    ['ja', 'ja'],
    ['ja-JP', 'ja'],
    ['ko', 'ko'],
    ['ko_KR', 'ko'],
    ['en', 'en'],
    ['en-US', 'en'],
    ['fr-FR', 'en'],
    ['', 'en'],
    ['  ja  ', 'ja'],
  ])('%s 归并成 %s', (raw, want) => {
    expect(normalizeLanguage(raw)).toBe(want)
  })

  it('不会把 zhx、jax 这类只是前缀相同的标识当成中文或日文', () => {
    expect(normalizeLanguage('zhx')).toBe('en')
    expect(normalizeLanguage('jax')).toBe('en')
    expect(normalizeLanguage('korean')).toBe('en')
  })
})

describe('pickLanguage', () => {
  it('按偏好顺序挑第一个支持的：首选法语、其次日语时给日语', () => {
    expect(pickLanguage(['fr-FR', 'ja'])).toBe('ja')
  })

  it('首选就支持时用首选', () => {
    expect(pickLanguage(['zh-CN', 'en'])).toBe('zh-CN')
    expect(pickLanguage(['zh-Hant-TW', 'en'])).toBe('zh-TW')
  })

  it('一个都不支持或者没有候选时用英文', () => {
    expect(pickLanguage(['de', 'fr'])).toBe('en')
    expect(pickLanguage([])).toBe('en')
  })

  it('只是前缀相同的不算支持，要继续往后找', () => {
    expect(pickLanguage(['zhx', 'ko-KR'])).toBe('ko')
  })
})

describe('LANGUAGES', () => {
  it('正好是五种语言，代码和名称都不重复', () => {
    expect(LANGUAGES.map((l) => l.code).sort()).toEqual(['en', 'ja', 'ko', 'zh-CN', 'zh-TW'])
    expect(new Set(LANGUAGES.map((l) => l.label)).size).toBe(5)
  })

  it('每个语言代码归并后还是它自己', () => {
    for (const { code } of LANGUAGES) expect(normalizeLanguage(code)).toBe(code)
  })
})
