// 版本号的比较，以及「此版本忽略」在浏览器里的记录。

/** 记录被忽略的最高版本号的 localStorage 键；小于等于它的版本不再弹窗，换一台电脑仍会提醒 */
export const IGNORED_KEY = 'slate.update.ignored'

/** 把 v1.2.3 解析成三段数字；后面的预发布信息（-rc1）忽略，格式不对返回 null */
export function parseVersion(text: string): [number, number, number] | null {
  const core = text.trim().replace(/^v/, '').split(/[-+]/)[0]
  const parts = core.split('.')
  if (parts.length !== 3 || !parts.every((p) => /^\d+$/.test(p))) return null
  return [Number(parts[0]), Number(parts[1]), Number(parts[2])]
}

/** a 是否比 b 新；任何一个解析不了都算「不新」，宁可不弹窗也不乱弹 */
export function isNewer(a: string, b: string): boolean {
  const x = parseVersion(a)
  const y = parseVersion(b)
  if (!x || !y) return false
  for (let i = 0; i < 3; i++) {
    if (x[i] !== y[i]) return x[i] > y[i]
  }
  return false
}

/** 读出被忽略的版本号；浏览器禁用存储时当作没有 */
export function readIgnored(): string {
  try {
    return localStorage.getItem(IGNORED_KEY) ?? ''
  } catch {
    return ''
  }
}

/** 记下被忽略的版本号；写不进去（隐私模式、存储已满）时只在本次页面里生效，不报错 */
export function saveIgnored(version: string): void {
  try {
    localStorage.setItem(IGNORED_KEY, version)
  } catch {
    // 忽略：下次打开页面会再提醒一次，不影响使用
  }
}
