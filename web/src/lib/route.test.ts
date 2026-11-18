import { describe, expect, it } from 'vitest'
import { parseId } from './route'

describe('parseId', () => {
  it.each([
    ['1', 1],
    ['42', 42],
    ['123456789012345', 123456789012345],
  ])('%s 是合法的编号', (raw, want) => {
    expect(parseId(raw)).toBe(want)
  })

  it.each([
    [undefined],
    [null],
    [''],
    ['0'],
    ['-1'],
    ['01'],
    ['1.5'],
    ['1e3'],
    ['abc'],
    ['12abc'],
    [' 12'],
    ['12 '],
    ['+5'],
    ['0x10'],
    ['1234567890123456'],
    ['99999999999999999999'],
  ])('%s 不是合法的编号', (raw) => {
    expect(parseId(raw)).toBeNull()
  })
})
