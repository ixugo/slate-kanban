import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import { DescriptionEditor } from './DescriptionEditor'

describe('DescriptionEditor', () => {
  it('没有描述时显示占位按钮，点一下进入编辑', async () => {
    render(<DescriptionEditor value="" onSave={() => {}} />)
    await userEvent.click(screen.getByRole('button', { name: en.card.descPlaceholder }))
    expect(await screen.findByRole('textbox', { name: en.card.field.description })).toBeInTheDocument()
  })

  it('点渲染好的描述进入编辑，取消后不保存', async () => {
    const onSave = vi.fn()
    render(<DescriptionEditor value="**粗**" onSave={onSave} />)
    await userEvent.click(screen.getByText('粗'))
    const editor = await screen.findByRole('textbox', { name: en.card.field.description })
    expect(editor).toHaveTextContent('**粗**')
    await userEvent.click(screen.getByRole('button', { name: en.common.cancel }))
    expect(onSave).not.toHaveBeenCalled()
    expect(screen.queryByRole('textbox', { name: en.card.field.description })).toBeNull()
  })

  it('没改动就保存不会发请求；改了才保存原文', async () => {
    const onSave = vi.fn()
    render(<DescriptionEditor value="a" onSave={onSave} />)
    await userEvent.click(screen.getByRole('button', { name: en.card.descEdit }))
    const editor = await screen.findByRole('textbox', { name: en.card.field.description })
    await userEvent.click(screen.getByRole('button', { name: en.common.save }))
    expect(onSave).not.toHaveBeenCalled()

    await userEvent.click(screen.getByRole('button', { name: en.card.descEdit }))
    const again = await screen.findByRole('textbox', { name: en.card.field.description })
    expect(again).not.toBe(editor)
    again.dispatchEvent(new InputEvent('beforeinput', { inputType: 'insertText', data: 'b', cancelable: true, bubbles: true }))
    await userEvent.click(screen.getByRole('button', { name: en.common.save }))
    expect(onSave).toHaveBeenCalledWith('ab')
  })

  it('查看状态下点任务框直接保存改过的原文，点链接不进入编辑', async () => {
    const onSave = vi.fn()
    render(<DescriptionEditor value={'- [ ] 写文档\n\n[官网](https://a.com)'} onSave={onSave} />)
    await userEvent.click(screen.getByRole('checkbox'))
    expect(onSave).toHaveBeenCalledWith('- [x] 写文档\n\n[官网](https://a.com)')
    await userEvent.click(screen.getByRole('link', { name: '官网' }))
    expect(screen.queryByRole('textbox', { name: en.card.field.description })).toBeNull()
  })
})
