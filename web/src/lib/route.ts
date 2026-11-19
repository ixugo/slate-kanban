// 路由参数的解析：地址栏里的编号是用户可以随便改的输入，必须严格校验后才能当成数字用。

/** 编号最多的位数：15 位以内的整数在 JavaScript 里不会丢精度 */
const MAX_ID_DIGITS = 15

// 合法编号的写法：不以 0 开头的 1 到 15 位数字
const ID_PATTERN = new RegExp(`^[1-9]\\d{0,${MAX_ID_DIGITS - 1}}$`)

/** 把地址里的编号解析成正整数；不是纯数字、以 0 开头、是 0、太长、缺失都返回 null */
export function parseId(raw: string | null | undefined): number | null {
  if (!raw || !ID_PATTERN.test(raw)) return null
  return Number(raw)
}
