// 日期时间的显示：全部用浏览器自带的 Intl，跟随当前界面语言，不引入日期库。

/** 把 RFC3339 字符串解析成毫秒时间戳；空值或格式不对返回 null */
export function parseTime(iso: string | null | undefined): number | null {
  if (!iso) return null
  const ms = Date.parse(iso)
  return Number.isNaN(ms) ? null : ms
}

/** 完整的日期加时间，例如 2026/11/18 09:30；解析不了返回空串 */
export function formatDateTime(iso: string | null | undefined, lang: string): string {
  const ms = parseTime(iso)
  if (ms === null) return ''
  return new Intl.DateTimeFormat(lang, { dateStyle: 'medium', timeStyle: 'short' }).format(ms)
}

/** 只有日期，任务上的截止日期用它；解析不了返回空串 */
export function formatDate(iso: string | null | undefined, lang: string): string {
  const ms = parseTime(iso)
  if (ms === null) return ''
  return new Intl.DateTimeFormat(lang, { dateStyle: 'medium' }).format(ms)
}

/** 相对时间的换算档位：秒数上限、单位、一个单位对应多少秒（从小到大排列） */
const RELATIVE_STEPS: { limit: number; unit: Intl.RelativeTimeFormatUnit; seconds: number }[] = [
  { limit: 60, unit: 'second', seconds: 1 },
  { limit: 3_600, unit: 'minute', seconds: 60 },
  { limit: 86_400, unit: 'hour', seconds: 3_600 },
  { limit: 2_592_000, unit: 'day', seconds: 86_400 },
  { limit: 31_536_000, unit: 'month', seconds: 2_592_000 },
  { limit: Infinity, unit: 'year', seconds: 31_536_000 },
]

/** 相对现在的时间，例如"3 小时前"、"2 天后"（向下取整）；5 秒以内显示"现在"；解析不了返回空串 */
export function formatRelative(iso: string | null | undefined, lang: string, now: number = Date.now()): string {
  const ms = parseTime(iso)
  if (ms === null) return ''
  const diffSeconds = Math.round((ms - now) / 1000)
  const rtf = new Intl.RelativeTimeFormat(lang, { numeric: 'auto' })
  if (Math.abs(diffSeconds) < 5) return rtf.format(0, 'second')
  const abs = Math.abs(diffSeconds)
  const step = RELATIVE_STEPS.find((s) => abs < s.limit) ?? RELATIVE_STEPS[RELATIVE_STEPS.length - 1]
  // 向下取整：90 秒显示"1 分钟前"，3599 秒显示"59 分钟前"，过去和未来对称
  return rtf.format(Math.sign(diffSeconds) * Math.floor(abs / step.seconds), step.unit)
}

/** 这个时间是否已经过去（用来标红逾期的截止日期）；解析不了算没过 */
export function isPast(iso: string | null | undefined, now: number = Date.now()): boolean {
  const ms = parseTime(iso)
  return ms !== null && ms < now
}

/** 把 RFC3339 时间转成 <input type="datetime-local"> 需要的本地时间 "YYYY-MM-DDTHH:mm"；解析不了返回空串 */
export function toLocalInput(iso: string | null | undefined): string {
  const ms = parseTime(iso)
  if (ms === null) return ''
  const d = new Date(ms)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 把 <input type="datetime-local"> 的本地时间转成 RFC3339（UTC）；空串表示清除，原样返回空串；格式不对返回 null */
export function fromLocalInput(local: string): string | null {
  if (local === '') return ''
  const ms = new Date(local).getTime()
  return Number.isNaN(ms) ? null : new Date(ms).toISOString()
}
