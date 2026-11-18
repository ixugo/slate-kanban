import { describe, expect, it } from 'vitest'
import { readPort } from './env'

describe('readPort', () => {
  it('没设置或设成空串时用默认端口 17420', () => {
    expect(readPort(undefined)).toBe(17420)
    expect(readPort('')).toBe(17420)
  })

  it('接受 1024 到 65535 的整数，边界值也算', () => {
    expect(readPort('1024')).toBe(1024)
    expect(readPort('18000')).toBe(18000)
    expect(readPort('65535')).toBe(65535)
  })

  // 超出范围、位数太多、带小数或科学计数、带空白或符号、全角数字、不是数字，都要拒绝
  const invalid = [
    '1023', '65536', '0', '-1', '123456',
    '12.5', '2000.', '1e4', '8e3', '0x1F90',
    ' 8080', '8080 ', '+8080', '００８０',
    'abc',
  ]
  it.each(invalid)('拒绝不合法的端口 %j', (raw) => {
    expect(() => readPort(raw)).toThrow('E2E_PORT')
  })

  it('报错信息里只带取值的前 20 个字符，不把超长输入原样打出来', () => {
    let message = ''
    try {
      readPort('x'.repeat(500))
    } catch (err) {
      message = (err as Error).message
    }
    expect(message).toContain('E2E_PORT')
    expect(message.length).toBeLessThan(100)
  })
})
