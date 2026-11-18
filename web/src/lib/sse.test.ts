import { describe, expect, it } from 'vitest'
import { SseParser, type SseMessage } from './sse'

// 把一段完整文本一次喂给解析器，返回全部消息。
function parseAll(text: string): SseMessage[] {
  return new SseParser().feed(text)
}

// 把文本按给定的长度切成很多块再喂，模拟网络把数据从任意位置切断。
function parseInChunks(text: string, size: number): SseMessage[] {
  const parser = new SseParser()
  const out: SseMessage[] = []
  for (let i = 0; i < text.length; i += size) out.push(...parser.feed(text.slice(i, i + size)))
  return out
}

const sample =
  'event: ready\ndata: {"latest":3}\n\n' +
  ': ping\n\n' +
  'event: board\nid: 4\ndata: {"id":4}\n\n' +
  'data: line1\ndata: line2\n\n'

describe('SseParser', () => {
  it('解析事件名、数据和 id', () => {
    expect(parseAll('event: board\nid: 7\ndata: {"id":7}\n\n')).toEqual([
      { event: 'board', data: '{"id":7}', id: '7' },
    ])
  })

  it('没有 event 行时事件名是 message，没有 id 时是空串', () => {
    expect(parseAll('data: hi\n\n')).toEqual([{ event: 'message', data: 'hi', id: '' }])
  })

  it('多个 data 行用换行连起来', () => {
    expect(parseAll('data: a\ndata: b\ndata:\ndata: c\n\n')[0].data).toBe('a\nb\n\nc')
  })

  it('冒号后只去掉一个空格', () => {
    expect(parseAll('data:  two spaces\n\n')[0].data).toBe(' two spaces')
    expect(parseAll('data:nospace\n\n')[0].data).toBe('nospace')
  })

  it('注释行（心跳）被忽略，不产生消息', () => {
    expect(parseAll(': ping\n\n: another\n\n')).toEqual([])
  })

  it('只有 event 没有 data 的块被丢弃，并且不影响下一条消息的事件名', () => {
    expect(parseAll('event: lonely\n\ndata: x\n\n')).toEqual([{ event: 'message', data: 'x', id: '' }])
  })

  it('每条消息的 id 互不影响：后一条没写 id 就是空串', () => {
    const msgs = parseAll('id: 1\ndata: a\n\ndata: b\n\n')
    expect(msgs.map((m) => m.id)).toEqual(['1', ''])
  })

  it('三种换行符都认：\\n、\\r\\n、\\r', () => {
    expect(parseAll('data: a\r\n\r\n')).toEqual([{ event: 'message', data: 'a', id: '' }])
    expect(parseAll('data: a\r\r')).toEqual([{ event: 'message', data: 'a', id: '' }])
    expect(parseAll('data: a\n\n')).toEqual([{ event: 'message', data: 'a', id: '' }])
  })

  it('\\r\\n 结尾的多行数据仍然是一条消息，每个 \\r\\n 只算一个换行', () => {
    expect(parseAll('data: a\r\ndata: b\r\n\r\n')).toEqual([{ event: 'message', data: 'a\nb', id: '' }])
  })

  it('\\r 和 \\n 被切在两块之间时，多行数据也不会被拆成两条消息', () => {
    const parser = new SseParser()
    const out = [...parser.feed('data: a\r'), ...parser.feed('\ndata: b\r\n\r\n')]
    expect(out).toEqual([{ event: 'message', data: 'a\nb', id: '' }])
  })

  it('开头的字节序标记被去掉', () => {
    expect(parseAll('\uFEFFdata: a\n\n')).toEqual([{ event: 'message', data: 'a', id: '' }])
  })

  it('不认识的字段和 retry 被忽略', () => {
    expect(parseAll('retry: 1000\nfoo: bar\ndata: a\n\n')).toEqual([{ event: 'message', data: 'a', id: '' }])
  })

  it('包含空字符的 id 被忽略', () => {
    expect(parseAll('id: a\0b\ndata: x\n\n')[0].id).toBe('')
  })

  it('没有结尾空行的消息不输出，等后面的数据补上', () => {
    const parser = new SseParser()
    expect(parser.feed('data: partial\n')).toEqual([])
    expect(parser.feed('\n')).toEqual([{ event: 'message', data: 'partial', id: '' }])
  })

  it('不管怎么切块，结果都和一次喂入一样', () => {
    const whole = parseAll(sample)
    expect(whole).toHaveLength(3)
    for (const size of [1, 2, 3, 5, 7, 13]) {
      expect(parseInChunks(sample, size)).toEqual(whole)
    }
  })

  it('\\r\\n 被切在两块之间时只算一个换行', () => {
    const parser = new SseParser()
    const out = [...parser.feed('data: a\r'), ...parser.feed('\n\r'), ...parser.feed('\n')]
    expect(out).toEqual([{ event: 'message', data: 'a', id: '' }])
  })

  it('一块里有多条消息时按顺序全部输出', () => {
    const msgs = parseAll('data: 1\n\ndata: 2\n\ndata: 3\n\n')
    expect(msgs.map((m) => m.data)).toEqual(['1', '2', '3'])
  })
})
