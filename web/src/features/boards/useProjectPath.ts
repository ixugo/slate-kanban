// 创建项目和常规设置共用的工作目录校验。每次编辑都使上一次校验失效。

import { useEffect, useRef, useState } from 'react'
import { ApiError } from '@/lib/api/client'
import { checkProjectPath } from '@/lib/api/endpoints'

type CheckFailure = 'missing' | 'invalid' | 'failed' | 'empty'
type CheckStatus = 'checking' | 'valid' | CheckFailure

export interface UseProjectPathOptions {
  /** 桌面客户端里工作目录必填；网页端选填。 */
  required?: boolean
}

function checkErrorStatus(error: unknown): CheckFailure {
  if (error instanceof ApiError) {
    if (error.reason === 'ErrProjectPathNotFound') return 'missing'
    if (error.reason === 'ErrInvalidInput') return 'invalid'
  }
  return 'failed'
}

export function useProjectPath(initialValue = '', options?: UseProjectPathOptions) {
  const required = options?.required ?? false
  // 尚未编辑时直接回显服务端的值，避免覆盖 MCP 或其他页面新写入的路径。
  const [draft, setDraft] = useState<string>()
  const [check, setCheck] = useState<{ path: string; status: CheckStatus }>()
  const request = useRef<AbortController | null>(null)
  const value = draft ?? initialValue
  const trimmed = value.trim()
  const changed = draft !== undefined && trimmed !== initialValue
  const status = check?.path === trimmed ? check.status : 'unchecked'
  const emptyRequired = required && !trimmed
  const blocked = emptyRequired || (status !== 'unchecked' && status !== 'valid')

  useEffect(() => () => request.current?.abort(), [])

  const change = (next: string) => {
    request.current?.abort()
    request.current = null
    setDraft(next)
    setCheck(undefined)
  }

  const validate = async (): Promise<boolean> => {
    request.current?.abort()
    if (!trimmed) {
      request.current = null
      if (required) setCheck({ path: '', status: 'empty' })
      else setCheck(undefined)
      return !required
    }
    const controller = new AbortController()
    request.current = controller
    setCheck({ path: trimmed, status: 'checking' })
    try {
      await checkProjectPath(trimmed, controller.signal)
      if (controller.signal.aborted) return false
      setCheck({ path: trimmed, status: 'valid' })
      return true
    } catch (error) {
      // 即使旧请求不遵守 AbortSignal，也不能影响新的输入或放行旧的保存。
      if (!controller.signal.aborted) setCheck({ path: trimmed, status: checkErrorStatus(error) })
      return false
    } finally {
      if (request.current === controller) request.current = null
    }
  }

  // 处理直接回车等未失焦的提交。没有编辑原有路径时不重新提交它，兼容外部 Agent 的路径。
  const ensureValid = async () => {
    if (blocked) return false
    if (required && !trimmed) return false
    if (!changed || !trimmed || status === 'valid') return true
    return validate()
  }

  // 目录可能在预校验后、真正保存前消失，保存接口的目录错误也显示在原字段下。
  const reportSaveError = (error: unknown) => {
    if (!(error instanceof ApiError)) return
    if (error.reason !== 'ErrProjectPathNotFound' && error.reason !== 'ErrProjectPathCheckFailed') return
    request.current?.abort()
    request.current = null
    setCheck({ path: trimmed, status: checkErrorStatus(error) })
  }

  const reset = () => {
    request.current?.abort()
    request.current = null
    setDraft(undefined)
    setCheck(undefined)
  }

  return { value, trimmed, changed, status, blocked, required, change, validate, ensureValid, reportSaveError, reset }
}