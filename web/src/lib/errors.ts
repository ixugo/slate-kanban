// 把接口错误整理成给人看的文字：按当前语言取 errors.<reason> 的翻译，
// 没有翻译才退回服务端的兜底文案（中文），再没有就显示"出了点问题"。

import type { TFunction } from 'i18next'
import { en } from '@/i18n/locales/en'
import { ApiError } from '@/lib/api/client'

/** 有翻译的错误名，errors 里除了 title 和 traceId 以外的键 */
type ErrorReason = Exclude<keyof typeof en.errors, 'title' | 'traceId'>

/** 整理后的错误文字 */
export interface ErrorText {
  /** 主要提示，已按当前语言翻译 */
  message: string
  /** 补充说明：校验类错误里服务端写明的具体原因（中文），其他错误为空串 */
  detail: string
  /** 服务端的追踪号，用来在日志里找到这次请求，没有为空串 */
  traceId: string
}

/** 这几类错误的服务端文案里写着具体哪个字段不对，值得原样补充显示 */
const DETAIL_REASONS = new Set(['ErrInvalidInput', 'ErrInvalidName', 'ErrBadRequest'])

// 判断错误名是否有翻译；title 和 traceId 不是错误名，要排除。
function isKnownReason(reason: string): reason is ErrorReason {
  return reason !== 'title' && reason !== 'traceId' && Object.hasOwn(en.errors, reason)
}

/** 请求被主动取消（切页面、切看板）：这不是错误，调用方不应该提示 */
export function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}

/** 把任意错误整理成 ErrorText；t 传当前语言的翻译函数 */
export function describeError(err: unknown, t: TFunction): ErrorText {
  if (err instanceof ApiError) {
    const known = isKnownReason(err.reason)
    const message = known ? t(`errors.${err.reason}`) : err.message || t('errors.title')
    const detail = known && DETAIL_REASONS.has(err.reason) && err.message !== message ? err.message : ''
    return { message, detail, traceId: err.traceId }
  }
  const raw = err instanceof Error ? err.message : ''
  return { message: raw || t('errors.title'), detail: '', traceId: '' }
}

/** 判断是不是"版本冲突"：卡片被别人先改过，页面要提示刷新而不是简单报错 */
export function isVersionConflict(err: unknown): boolean {
  return err instanceof ApiError && err.reason === 'ErrCardVersionConflict'
}
