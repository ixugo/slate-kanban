// 新建项目表单的校验：规则和后端 internal/core/kanban/board.param.go 一致，提前在输入框旁边提示。
// 后端仍然是最终的校验方，这里只是不让用户白等一次请求。

import { BOARD_NAME_MAX, SLUG_MAX, SLUG_MIN } from '@/lib/limits'

/** 表单里需要校验的几个字段的值 */
export interface BoardFormValues {
  name: string
  slug: string
}

/** 出错字段对应的文案键后缀（boards.create.errors.<后缀>） */
export type BoardFormErrors = Partial<Record<'name' | 'slug', 'nameRequired' | 'slugInvalid'>>

/** 编号前缀的写法：大写字母开头，后面是大写字母或数字，总长度 2 到 10 位 */
const SLUG_PATTERN = new RegExp(`^[A-Z][A-Z0-9]{${SLUG_MIN - 1},${SLUG_MAX - 1}}$`)

/** Windows 盘符路径，例如 C:\code 或 c:/code */
const WINDOWS_DRIVE = /^[A-Za-z]:[\\/]/

/** 是不是绝对路径：Unix 的 /、网络共享的 \\，或者 Windows 盘符，和后端 isAbsolutePath 一致；设置页的项目路径校验也用它 */
export function isAbsolutePath(path: string): boolean {
  return path.startsWith('/') || path.startsWith('\\\\') || WINDOWS_DRIVE.test(path)
}

/** 校验表单；返回空对象表示全部通过。名称按去掉首尾空白后判断，编号前缀可以留空 */
export function validateBoardForm(values: BoardFormValues): BoardFormErrors {
  const errors: BoardFormErrors = {}
  const name = values.name.trim()
  if (name === '' || Array.from(name).length > BOARD_NAME_MAX) errors.name = 'nameRequired'
  const slug = values.slug.trim()
  if (slug !== '' && !SLUG_PATTERN.test(slug.toUpperCase())) errors.slug = 'slugInvalid'
  return errors
}
