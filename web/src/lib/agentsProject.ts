/** 解析接入页 URL 的 project 参数；空串与缺省都视为未绑定看板。 */
export function parseAgentsProjectParam(raw: string | null): string | null {
  if (raw === null || raw.trim() === '') return null
  return raw.toUpperCase()
}

/** 根据当前看板 slug 生成顶栏 Agent 链用的查询串；列表未就绪时没有 slug 则不带参。 */
export function agentsSearchFromBoardSlug(slug: string | undefined): string {
  if (!slug) return ''
  return `?project=${slug.toLowerCase()}`
}