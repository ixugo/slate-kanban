// 撤销与重做的历史栈。所见即所得编辑器每次输入都会重画 DOM，浏览器自带的撤销栈用不了，所以自己管。

import type { EditState } from './commands'

/** 连续输入时，两次按键间隔不超过这个毫秒数就并入同一步撤销 */
const COALESCE_MS = 1000
/** 最多能撤销多少步；超出后丢掉最老的 */
const MAX_STEPS = 200

/** 一次改动是输入文字，还是命令（回车、格式、粘贴、删除选区等）；只有连续的输入会合并 */
export type ChangeKind = 'type' | 'command'

/** 改动之前的选区，撤销时要把光标放回去 */
interface Sel {
  anchor: number
  head: number
}

/** 历史里的一步：改动之后的状态，以及改动之前的选区 */
interface Step {
  state: EditState
  before: Sel
}

/** 撤销/重做栈 */
export class History {
  private steps: Step[]
  private index = 0
  private lastKind: ChangeKind = 'command'
  private lastAt = 0
  private readonly now: () => number

  /** initial 是编辑开始时的状态；now 用来取当前毫秒数，测试时可以换成假时钟 */
  constructor(initial: EditState, now: () => number = Date.now) {
    this.now = now
    this.steps = [{ state: initial, before: { anchor: initial.anchor, head: initial.head } }]
  }

  /**
   * 记录一次改动。before 是改动前的选区。
   * 文字没变就忽略；连续的输入（间隔短、光标没被挪走过）合并成一步，撤销时一次退回这一串输入之前。
   */
  push(state: EditState, kind: ChangeKind, before: Sel): void {
    const top = this.steps[this.index]
    if (state.value === top.state.value) return
    const at = this.now()
    const continuing =
      kind === 'type' &&
      this.lastKind === 'type' &&
      at - this.lastAt < COALESCE_MS &&
      before.anchor === before.head &&
      before.head === top.state.head
    this.lastKind = kind
    this.lastAt = at
    if (continuing) {
      top.state = state
      return
    }
    this.steps.length = this.index + 1
    this.steps.push({ state, before: { anchor: before.anchor, head: before.head } })
    this.index++
    if (this.steps.length > MAX_STEPS + 1) {
      this.steps.shift()
      this.index--
    }
  }

  /** 撤销一步：返回改动之前的文字，选区放回改动之前的位置；已经到头返回 null */
  undo(): EditState | null {
    if (this.index === 0) return null
    const { before } = this.steps[this.index]
    this.index--
    this.lastKind = 'command'
    return { value: this.steps[this.index].state.value, anchor: before.anchor, head: before.head }
  }

  /** 重做一步：返回改动之后的状态；没有可重做的返回 null */
  redo(): EditState | null {
    if (this.index >= this.steps.length - 1) return null
    this.index++
    this.lastKind = 'command'
    return this.steps[this.index].state
  }
}
