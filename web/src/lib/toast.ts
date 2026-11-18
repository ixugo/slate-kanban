// 轻提示的统一入口：toast.success / toast.error 这类函数式写法，底层走自研的 toast-store。

import { defaultDuration, dismissToast, pushToast, type ToastVariant } from './toast-store'

export interface ToastOptions {
  description?: string
  duration?: number
  id?: string | number
}

type ToastFn = (title: string, opts?: ToastOptions) => string | number

/** 构造某一变体的快捷方法 */
function variantToast(variant: ToastVariant): ToastFn {
  return (title, opts) =>
    pushToast({
      title,
      description: opts?.description,
      duration: opts?.duration ?? defaultDuration(variant),
      id: opts?.id,
      variant,
    })
}

/** 主入口：toast(title) 与 toast.success / error 等方法 */
function toast(title: string, opts?: ToastOptions): string | number {
  return pushToast({ title, description: opts?.description, duration: opts?.duration, id: opts?.id, variant: 'default' })
}

toast.success = variantToast('success')
toast.error = variantToast('error')
toast.info = variantToast('info')
toast.warning = variantToast('warning')
toast.message = toast
toast.dismiss = dismissToast

export { toast }