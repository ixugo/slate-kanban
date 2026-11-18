// SSE（text/event-stream）的增量解析器。
// 不用浏览器自带的 EventSource：它不能带 Authorization 请求头，令牌就只能放进 URL。
// 这里自己按规范解析 fetch 读到的字节流，读一块喂一块，吐出已经完整的消息。

/** 一条完整的 SSE 消息 */
export interface SseMessage {
  /** 事件名，没写 `event:` 时是 message */
  event: string
  /** 数据；多行 `data:` 用换行连接 */
  data: string
  /** 这条消息自己带的 `id:`，没带是空串 */
  id: string
}

/** 规范里的默认事件名 */
const DEFAULT_EVENT = 'message'

/** 文本流开头可能有的字节序标记 */
const BOM = '\uFEFF'

/**
 * 增量解析器。一次读到的数据可能把一行从中间切断，也可能把 `\r\n` 切在两块之间：
 * 没有结束的半行留在缓冲区里，等下一块到了再继续；块末尾的 `\r` 立刻当作换行处理，
 * 并记下"如果下一块以 `\n` 开头要跳过它"，这样 `\r\n` 不管怎么切都只算一个换行，也不会让消息迟迟发不出去。
 */
export class SseParser {
  private buffer = ''
  private started = false
  private skipLF = false
  private event = ''
  private dataLines: string[] = []
  private id = ''

  /** 喂入一块文本，返回这一块里新完整的消息（可能是 0 条、1 条或多条） */
  feed(chunk: string): SseMessage[] {
    if (this.skipLF && chunk.length > 0) {
      if (chunk.startsWith('\n')) chunk = chunk.slice(1)
      this.skipLF = false
    }
    this.buffer += chunk
    if (!this.started && this.buffer.length > 0) {
      this.started = true
      if (this.buffer.startsWith(BOM)) this.buffer = this.buffer.slice(1)
    }
    const out: SseMessage[] = []
    let start = 0
    for (let i = 0; i < this.buffer.length; i++) {
      const ch = this.buffer[i]
      if (ch !== '\n' && ch !== '\r') continue
      this.processLine(this.buffer.slice(start, i), out)
      if (ch === '\r') i += this.skipFollowingLF(i)
      start = i + 1
    }
    this.buffer = this.buffer.slice(start)
    return out
  }

  // 刚处理完下标 i 处的 \r：后面紧跟 \n 就一起跳过（返回 1）；\r 正好是块的最后一个字符，
  // 就记下下一块开头的 \n 要跳过（返回 0）。
  private skipFollowingLF(i: number): number {
    if (this.buffer[i + 1] === '\n') return 1
    if (i === this.buffer.length - 1) this.skipLF = true
    return 0
  }

  // 处理一行：空行表示一条消息结束；其余按 "字段: 值" 解析。
  // 冒号开头的注释行（心跳 `: ping`）解析出来字段名是空串，和不认识的字段一样被忽略，不需要单独判断。
  private processLine(line: string, out: SseMessage[]): void {
    if (line === '') {
      this.dispatch(out)
      return
    }
    const colon = line.indexOf(':')
    const field = colon === -1 ? line : line.slice(0, colon)
    let value = colon === -1 ? '' : line.slice(colon + 1)
    if (value.startsWith(' ')) value = value.slice(1)
    this.applyField(field, value)
  }

  // 记下一个字段。retry 由客户端自己决定退避时间，所以忽略；不认识的字段按规范忽略。
  private applyField(field: string, value: string): void {
    switch (field) {
      case 'event':
        this.event = value
        break
      case 'data':
        this.dataLines.push(value)
        break
      case 'id':
        if (!value.includes('\0')) this.id = value
        break
    }
  }

  // 一条消息结束：没有 data 的块（比如只有 event 行）按规范丢弃；有就输出，并清空状态准备下一条。
  private dispatch(out: SseMessage[]): void {
    if (this.dataLines.length > 0) {
      out.push({ event: this.event || DEFAULT_EVENT, data: this.dataLines.join('\n'), id: this.id })
    }
    this.event = ''
    this.dataLines = []
    this.id = ''
  }
}
