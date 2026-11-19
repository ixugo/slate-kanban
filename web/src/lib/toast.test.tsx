import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Toaster } from '@/components/ui/sonner'
import { toast } from '@/lib/toast'
import { dismissToast } from '@/lib/toast-store'

function renderToaster() {
  return render(<Toaster />)
}

describe('toast', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    dismissToast()
  })

  afterEach(() => {
    dismissToast()
    vi.useRealTimers()
  })

  it('显示标题和补充说明', () => {
    renderToaster()
    act(() => toast.success('保存成功', { description: '细节' }))
    const alert = screen.getByRole('status')
    expect(within(alert).getByText('保存成功')).toBeInTheDocument()
    expect(within(alert).getByText('细节')).toBeInTheDocument()
  })

  it('普通提示约 4 秒后自动消失', async () => {
    renderToaster()
    act(() => toast('短暂'))
    expect(screen.getByText('短暂')).toBeInTheDocument()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4001)
    })
    expect(screen.queryByText('短暂')).not.toBeInTheDocument()
  })

  it('错误提示约 8 秒后消失，并使用 role=alert', async () => {
    renderToaster()
    act(() => toast.error('失败'))
    const alert = screen.getByRole('alert')
    expect(within(alert).getByText('失败')).toBeInTheDocument()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(8001)
    })
    expect(screen.queryByText('失败')).not.toBeInTheDocument()
  })

  it('悬停时暂停倒计时', async () => {
    renderToaster()
    act(() => toast('悬停'))
    const row = screen.getByText('悬停').closest('li') as HTMLElement
    act(() => fireEvent.mouseEnter(row))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000)
    })
    expect(screen.getByText('悬停')).toBeInTheDocument()
    act(() => fireEvent.mouseLeave(row))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4000)
    })
    expect(screen.queryByText('悬停')).not.toBeInTheDocument()
  })

  it('手动关闭', () => {
    renderToaster()
    act(() => toast('可关'))
    const btn = screen.getByRole('button', { name: 'Close' })
    act(() => fireEvent.click(btn))
    expect(screen.queryByText('可关')).not.toBeInTheDocument()
  })

  it('多条堆叠', () => {
    renderToaster()
    act(() => {
      toast('第一条')
      toast('第二条')
    })
    expect(screen.getAllByRole('status')).toHaveLength(2)
  })

  it('相同 id 更新同一条', () => {
    renderToaster()
    let id: string | number = 0
    act(() => {
      id = toast('旧', { id: 'x' })
      toast('新', { id })
    })
    expect(screen.getByText('新')).toBeInTheDocument()
    expect(screen.queryByText('旧')).not.toBeInTheDocument()
    expect(screen.getAllByRole('status')).toHaveLength(1)
  })
})