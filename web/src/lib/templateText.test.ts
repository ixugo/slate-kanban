import { describe, expect, it } from 'vitest'
import type { Lang } from '@/i18n/languages'
import { localizeColumnDescription, localizeColumnName, localizeRuleName } from './templateText'

const LANGS: Lang[] = ['zh-CN', 'zh-TW', 'en', 'ja', 'ko']

describe('起步模板文案的换语言显示', () => {
  it.each(LANGS)('任意语言写下的列名都能换成 %s 的说法', (lang) => {
    expect(localizeColumnName('想法池', lang)).toBe({ 'zh-CN': '想法池', 'zh-TW': '想法池', en: 'Ideas', ja: 'アイデア', ko: '아이디어' }[lang])
    expect(localizeColumnName('In progress', lang)).toBe({ 'zh-CN': '进行中', 'zh-TW': '進行中', en: 'In progress', ja: '進行中', ko: '진행 중' }[lang])
    expect(localizeColumnName('待辦', lang)).toBe(localizeColumnName('To do', lang))
  })

  it('用户自己起的名字和改过的文字原样显示', () => {
    expect(localizeColumnName('Backlog', 'ja')).toBe('Backlog')
    expect(localizeColumnName('待办 ', 'en')).toBe('待办 ')
    expect(localizeColumnName('', 'en')).toBe('')
    expect(localizeColumnDescription('我们自己写的说明', 'en')).toBe('我们自己写的说明')
    expect(localizeRuleName('我的规则', 'en')).toBe('我的规则')
  })

  it('列说明和默认规则名也随语言变', () => {
    expect(localizeColumnDescription('确定要做、正在排队的任务。', 'en')).toBe('Tasks that are confirmed and waiting in line.')
    expect(localizeRuleName('开始计时', 'en')).toBe('Start timer')
    expect(localizeRuleName('Reopen', 'zh-TW')).toBe('重新開啟')
  })

  it('不同种类的文案不会重名，否则反查会串', () => {
    const seen = new Map<string, string>()
    for (const lang of LANGS) {
      for (const name of ['想法池', '待办', '进行中', '审查', '完成']) {
        const out = localizeColumnName(name, lang)
        expect(out).not.toBe('')
        seen.set(`${lang}:${name}`, out)
      }
    }
    for (const lang of LANGS) {
      const outs = ['想法池', '待办', '进行中', '审查', '完成'].map((n) => seen.get(`${lang}:${n}`))
      expect(new Set(outs).size).toBe(5)
    }
  })
})
