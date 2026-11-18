// 翻译键与 t 函数的类型：以英文文案结构为基准，编译期拦住写错的键。

import type { Messages } from './locales/en'
import { en } from './locales/en'

/** 把嵌套文案对象拍平成点分路径，例如 common.save */
type LeafPaths<T, Prefix extends string = ''> = T extends string
  ? Prefix
  : {
      [K in keyof T & string]: LeafPaths<T[K], Prefix extends '' ? K : `${Prefix}.${K}`>
    }[keyof T & string]

/** 静态翻译键：英文里每一个叶子字符串路径 */
export type TranslationKey = LeafPaths<Messages>

/** 领域错误 reason 对应的动态键，供 describeError 里 t(`errors.${reason}`) 使用 */
type ErrorReason = Exclude<keyof typeof en.errors, 'title' | 'traceId'>
export type ErrorKey = `errors.${ErrorReason}`

/** 组件和工具函数拿到的翻译函数签名 */
export type TFunction = {
  (key: TranslationKey, params?: Record<string, string | number>): string
  (key: ErrorKey, params?: Record<string, string | number>): string
}

/** 运行时挂在各语言上的文案树，形状与 Messages 一致 */
export type MessageTree = Messages