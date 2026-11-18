// 统一的出错提示：各处的变更操作失败时调用同一个函数，显示翻译好的原因和追踪号。

import { useCallback } from 'react'
import { useTranslation } from '@/i18n/react'
import { toast } from '@/lib/toast'
import { describeError, isAbortError } from './errors'

/** 返回一个函数，传入错误就弹出提示；主动取消的请求不提示 */
export function useErrorToast(): (err: unknown) => void {
  const { t } = useTranslation()
  return useCallback(
    (err: unknown) => {
      if (isAbortError(err)) return
      const text = describeError(err, t)
      const description = [text.detail, text.traceId ? t('errors.traceId', { id: text.traceId }) : '']
        .filter(Boolean)
        .join('\n')
      toast.error(text.message, { description: description || undefined })
    },
    [t],
  )
}
