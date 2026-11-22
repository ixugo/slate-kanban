// 任务链接字段校验：只允许 http 和 https。

/** 空串表示清除；非空必须是 http(s) URL */
export function normalizeCardUrl(raw: string): { ok: true; value: string } | { ok: false } {
  const trimmed = raw.trim()
  if (!trimmed) return { ok: true, value: '' }
  try {
    const url = new URL(trimmed)
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return { ok: false }
    return { ok: true, value: trimmed }
  } catch {
    return { ok: false }
  }
}