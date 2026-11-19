// 复制文本到剪贴板；失败时选中节点内容并提示用户手动复制。

import { toast } from '@/lib/toast'

/** 尝试写入剪贴板，失败则选中文本并返回 false */
export async function copyText(text: string, el?: HTMLElement | null): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    if (el) {
      const range = document.createRange()
      range.selectNodeContents(el)
      const sel = window.getSelection()
      sel?.removeAllRanges()
      sel?.addRange(range)
    }
    return false
  }
}

/** 复制并在失败时弹出说明 */
export async function copyWithToast(text: string, el: HTMLElement | null, ok: string, fail: string): Promise<void> {
  const done = await copyText(text, el)
  if (done) toast.success(ok)
  else toast.error(fail)
}