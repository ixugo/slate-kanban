import { describe, expect, it } from 'vitest'
import type { EditState } from './commands'
import { History } from './history'

/** 方便写用例：光标放在文字末尾 */
const at = (value: string, caret = value.length): EditState => ({ value, anchor: caret, head: caret })

/** 假时钟：每次 tick 推进指定毫秒 */
function clock() {
  let t = 10_000
  return { now: () => t, tick: (ms: number) => (t += ms) }
}

describe('History', () => {
  it('刚开始没有可撤销也没有可重做的', () => {
    const h = new History(at(''))
    expect(h.undo()).toBeNull()
    expect(h.redo()).toBeNull()
  })

  it('撤销回到改动之前的文字，光标放回改动之前的位置；重做回到改动之后', () => {
    const h = new History(at('ab', 1))
    h.push(at('a**b**', 6), 'command', { anchor: 1, head: 2 })
    expect(h.undo()).toEqual({ value: 'ab', anchor: 1, head: 2 })
    expect(h.undo()).toBeNull()
    expect(h.redo()).toEqual(at('a**b**', 6))
    expect(h.redo()).toBeNull()
  })

  it('文字没变的改动不记录', () => {
    const h = new History(at('a'))
    h.push(at('a', 0), 'command', { anchor: 1, head: 1 })
    expect(h.undo()).toBeNull()
  })

  it('连续输入合并成一步撤销', () => {
    const c = clock()
    const h = new History(at(''), c.now)
    h.push(at('a'), 'type', { anchor: 0, head: 0 })
    c.tick(200)
    h.push(at('ab'), 'type', { anchor: 1, head: 1 })
    c.tick(200)
    h.push(at('abc'), 'type', { anchor: 2, head: 2 })
    expect(h.undo()).toEqual(at('', 0))
    expect(h.undo()).toBeNull()
    expect(h.redo()).toEqual(at('abc'))
  })

  it('停顿超过一秒再输入，算新的一步', () => {
    const c = clock()
    const h = new History(at(''), c.now)
    h.push(at('a'), 'type', { anchor: 0, head: 0 })
    c.tick(1500)
    h.push(at('ab'), 'type', { anchor: 1, head: 1 })
    expect(h.undo()).toEqual(at('a', 1))
    expect(h.undo()).toEqual(at('', 0))
  })

  it('光标被挪走再输入，算新的一步', () => {
    const c = clock()
    const h = new History(at('xy'), c.now)
    h.push(at('xya', 3), 'type', { anchor: 2, head: 2 })
    c.tick(100)
    h.push(at('bxya', 1), 'type', { anchor: 0, head: 0 })
    expect(h.undo()).toEqual(at('xya', 0))
  })

  it('中间插入一条命令会打断输入的合并', () => {
    const c = clock()
    const h = new History(at(''), c.now)
    h.push(at('a'), 'type', { anchor: 0, head: 0 })
    c.tick(100)
    h.push(at('a\n\n', 3), 'command', { anchor: 1, head: 1 })
    c.tick(100)
    h.push(at('a\n\nb'), 'type', { anchor: 3, head: 3 })
    expect(h.undo()).toEqual(at('a\n\n', 3))
    expect(h.undo()).toEqual(at('a', 1))
    expect(h.undo()).toEqual(at('', 0))
  })

  it('撤销之后再输入，不会并进撤销之前的那一串；也会丢掉可重做的分支', () => {
    const c = clock()
    const h = new History(at(''), c.now)
    h.push(at('a'), 'type', { anchor: 0, head: 0 })
    h.undo()
    c.tick(50)
    h.push(at('b'), 'type', { anchor: 0, head: 0 })
    expect(h.redo()).toBeNull()
    expect(h.undo()).toEqual(at('', 0))
  })

  it('选区被替换的输入不并进上一串', () => {
    const c = clock()
    const h = new History(at('abc'), c.now)
    h.push(at('abcd'), 'type', { anchor: 3, head: 3 })
    c.tick(50)
    h.push(at('abX', 3), 'type', { anchor: 1, head: 3 })
    expect(h.undo()).toEqual({ value: 'abcd', anchor: 1, head: 3 })
  })

  it('最多保留 200 步，更早的会被丢掉', () => {
    const h = new History(at(''))
    for (let i = 1; i <= 250; i++) h.push(at('x'.repeat(i)), 'command', { anchor: i - 1, head: i - 1 })
    let undone = 0
    while (h.undo()) undone++
    expect(undone).toBe(200)
  })
})
