import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import { AddCard } from './AddCard'

describe('AddCard', () => {
  it('展开后输入框自动获得焦点，不用再点一下就能打字', () => {
    render(<AddCard open onOpen={() => {}} onClose={() => {}} onSubmit={() => {}} />)
    expect(screen.getByRole('textbox', { name: en.board.addCard })).toHaveFocus()
  })

  it('提交中输入框只读但保持焦点，Enter 提交去掉首尾空白', async () => {
    const onSubmit = vi.fn()
    const { rerender } = render(<AddCard open onOpen={() => {}} onClose={() => {}} onSubmit={onSubmit} />)
    const box = screen.getByRole('textbox', { name: en.board.addCard })
    await userEvent.type(box, '  新卡片 {Enter}')
    expect(onSubmit).toHaveBeenCalledWith('新卡片')
    rerender(<AddCard open busy onOpen={() => {}} onClose={() => {}} onSubmit={onSubmit} />)
    expect(box).toHaveAttribute('readonly')
    expect(box).not.toBeDisabled()
    expect(box).toHaveFocus()
  })
})
