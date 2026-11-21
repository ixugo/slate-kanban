import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { DiffView } from './DiffView'

const base = {
  oldTitle: '修改前',
  newTitle: '修改后',
  identicalText: '内容一致',
}

describe('DiffView', () => {
  it('相同文本只显示 identicalText，不出现 diff 行', () => {
    render(<DiffView {...base} oldText={'a\nb'} newText={'a\nb'} />)
    expect(screen.getByText('内容一致')).toBeInTheDocument()
    expect(document.querySelector('[data-kind]')).toBeNull()
  })

  it('标题与增删统计', () => {
    render(<DiffView {...base} oldText="old" newText="new" />)
    expect(screen.getByText('修改前')).toBeInTheDocument()
    expect(screen.getByText('修改后')).toBeInTheDocument()
    expect(screen.getByText('+1')).toBeInTheDocument()
    expect(screen.getByText('−1')).toBeInTheDocument()
  })

  it('各行 data-kind 与行号：填充侧不显示行号', () => {
    render(<DiffView {...base} oldText="deleted" newText="" />)
    const row = document.querySelector('[data-kind="delete"]')
    expect(row).toBeTruthy()
    expect(screen.getByText('1')).toBeInTheDocument()
  })

  it('不会把尖括号文本当 HTML 解析', () => {
    render(<DiffView {...base} oldText="<b>x</b>" newText="<b>y</b>" />)
    expect(screen.getByText('<b>x</b>', { exact: false })).toBeInTheDocument()
    expect(document.querySelector('b')).toBeNull()
  })

  it('新增行右侧有行号、左侧无行号', () => {
    render(<DiffView {...base} oldText="" newText="only new" />)
    expect(document.querySelector('[data-kind="insert"]')).toBeTruthy()
    expect(screen.getByText('1')).toBeInTheDocument()
  })
})