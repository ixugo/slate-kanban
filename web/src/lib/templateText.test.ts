import { describe, expect, it } from 'vitest'
import type { Lang } from '@/i18n/languages'
import { localizeColumnName, localizeRuleName } from './templateText'

const LANGS: Lang[] = ['zh-CN', 'zh-TW', 'en', 'ja']

describe('起步模板文案的换语言显示', () => {
  it.each(LANGS)('任意语言写下的列名都能换成 %s 的说法', (lang) => {
    expect(localizeColumnName('收件箱', lang)).toBe({ 'zh-CN': '收件箱', 'zh-TW': '收件匣', en: 'Inbox', ja: '受信トレイ' }[lang])
    expect(localizeColumnName('In progress', lang)).toBe({ 'zh-CN': '进行中', 'zh-TW': '進行中', en: 'In progress', ja: '進行中' }[lang])
    expect(localizeColumnName('待辦', lang)).toBe(localizeColumnName('To do', lang))
  })

  it('用户自己起的名字和改过的文字原样显示', () => {
    expect(localizeColumnName('Backlog', 'ja')).toBe('Backlog')
    expect(localizeColumnName('待办 ', 'en')).toBe('待办 ')
    expect(localizeColumnName('', 'en')).toBe('')
    expect(localizeRuleName('我的规则', 'en')).toBe('我的规则')
  })

  it('列说明和默认规则名也随语言变', () => {
    expect(localizeRuleName('开始计时', 'en')).toBe('Start timer')
    expect(localizeRuleName('Reopen', 'zh-TW')).toBe('重新開啟')
  })

  it('不同种类的文案不会重名，否则反查会串', () => {
    const seen = new Map<string, string>()
    for (const lang of LANGS) {
      for (const name of ['收件箱', '待办', '进行中', '审查', '完成']) {
        const out = localizeColumnName(name, lang)
        expect(out).not.toBe('')
        seen.set(`${lang}:${name}`, out)
      }
    }
    for (const lang of LANGS) {
      const outs = ['收件箱', '待办', '进行中', '审查', '完成'].map((n) => seen.get(`${lang}:${n}`))
      expect(new Set(outs).size).toBe(5)
    }
  })
})
