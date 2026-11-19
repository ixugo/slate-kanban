// 在文案树里按点分路径取字符串，并做 {{name}} 占位符替换。

/** 按点分路径在嵌套对象里找叶子字符串；找不到返回 undefined */
export function getByPath(tree: object, path: string): string | undefined {
  let cur: unknown = tree
  for (const part of path.split('.')) {
    if (cur === null || typeof cur !== 'object' || !(part in cur)) return undefined
    cur = (cur as Record<string, unknown>)[part]
  }
  return typeof cur === 'string' ? cur : undefined
}

/**
 * 替换 {{name}} 占位符；没传参时原样返回；未提供的占位符保留，方便在界面上发现漏传。
 */
export function applyParams(text: string, params?: Record<string, string | number>): string {
  if (!params) return text
  return text.replaceAll(/\{\{\s*(\w+)\s*\}\}/g, (full, name: string) =>
    Object.hasOwn(params, name) ? String(params[name]) : full,
  )
}

/** 先取文案再替换占位符；缺失时返回 undefined */
export function resolveMessage(tree: object, path: string, params?: Record<string, string | number>): string | undefined {
  const raw = getByPath(tree, path)
  return raw === undefined ? undefined : applyParams(raw, params)
}