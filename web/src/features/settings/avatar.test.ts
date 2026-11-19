import { describe, expect, it } from 'vitest'
import { avatarTraits, randomAvatarSeed } from './avatar'

describe('avatarTraits', () => {
  it('同一个种子永远得到同一组特征', () => {
    expect(avatarTraits('e1')).toEqual(avatarTraits('e1'))
  })

  it('特征都在取值范围内', () => {
    for (let i = 0; i < 200; i++) {
      const t = avatarTraits(`e${i}`)
      expect(t.hue).toBeGreaterThanOrEqual(0)
      expect(t.hue).toBeLessThan(360)
      expect(t.hue2).toBeGreaterThanOrEqual(0)
      expect(t.hue2).toBeLessThan(360)
      expect([0, 1, 2]).toContain(t.eyes)
      expect([0, 1, 2]).toContain(t.mouth)
    }
  })

  it('相邻编号的头像不会都一样，五官样式各种取值都会出现', () => {
    const traits = Array.from({ length: 60 }, (_, i) => avatarTraits(`e${i + 1}`))
    expect(new Set(traits.map((t) => t.hue)).size).toBeGreaterThan(30)
    expect(new Set(traits.map((t) => t.eyes)).size).toBe(3)
    expect(new Set(traits.map((t) => t.mouth)).size).toBe(3)
  })

  it('两种画风都会出现，真人比猫多', () => {
    const count: Record<string, number> = {}
    for (let i = 0; i < 300; i++) {
      const { style } = avatarTraits(`seed${i}`)
      count[style] = (count[style] ?? 0) + 1
    }
    expect(Object.keys(count).sort()).toEqual(['cat', 'person'])
    expect(count.person).toBeGreaterThan(count.cat)
  })
})

describe('randomAvatarSeed', () => {
  it('只含小写字母和数字，1 到 8 位，连着生成的结果不同', () => {
    const seeds = Array.from({ length: 20 }, randomAvatarSeed)
    for (const seed of seeds) expect(seed).toMatch(/^[a-z0-9]{1,8}$/)
    expect(new Set(seeds).size).toBeGreaterThan(15)
  })
})
