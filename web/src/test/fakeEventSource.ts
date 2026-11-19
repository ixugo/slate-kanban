// 测试里模拟 EventSource，供看板页等集成测试使用。

import { vi } from 'vitest'

/** 可手动触发事件的假 EventSource */
export class FakeEventSource {
  static instances: FakeEventSource[] = []
  static readonly CLOSED = 2
  readonly url: string
  readyState = 1
  closed = false
  private listeners = new Map<string, Array<(e: Event) => void>>()

  constructor(url: string) {
    this.url = url
    FakeEventSource.instances.push(this)
  }

  addEventListener(type: string, fn: (e: Event) => void) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn])
  }

  close() {
    this.closed = true
    this.readyState = FakeEventSource.CLOSED
  }

  emit(type: string, data?: unknown) {
    const event = new MessageEvent(type, { data: data === undefined ? undefined : JSON.stringify(data) })
    this.listeners.get(type)?.forEach((fn) => fn(event))
  }
}

/** 在全局安装 FakeEventSource */
export function installFakeEventSource() {
  FakeEventSource.instances = []
  vi.stubGlobal('EventSource', FakeEventSource)
}