// 前端体积预算检查：构建之后运行，统计 gzip 后的大小，超出预算就以非零状态退出。
// 预算见 .cursor/scopes/v1.scope.md：首屏 JS 不超过 130 kB，整个 dist 不超过 300 kB（都是 gzip 后）。

import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, extname } from 'node:path'
import { gzipSync } from 'node:zlib'

const DIST = new URL('../../internal/web/spa/dist/', import.meta.url).pathname
/** 首屏 JS（index.html 里直接引用的脚本）gzip 后的上限，单位字节 */
const ENTRY_JS_BUDGET = 130 * 1024
/** 整个 dist 目录 gzip 后的上限，单位字节 */
const TOTAL_BUDGET = 300 * 1024

// 递归列出目录里的所有文件
function walk(dir) {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name)
    return statSync(full).isDirectory() ? walk(full) : [full]
  })
}

// 文件 gzip 之后的字节数
function gzipSize(file) {
  return gzipSync(readFileSync(file), { level: 9 }).length
}

const kb = (n) => `${(n / 1024).toFixed(1)} kB`
const files = walk(DIST)
const html = readFileSync(join(DIST, 'index.html'), 'utf8')
const entryScripts = [...html.matchAll(/<(?:script|link)[^>]+(?:src|href)="\/(assets\/[^"]+\.js)"/g)].map((m) => join(DIST, m[1]))
const entryJs = entryScripts.reduce((sum, f) => sum + gzipSize(f), 0)
const total = files.reduce((sum, f) => sum + (['.map'].includes(extname(f)) ? 0 : gzipSize(f)), 0)

console.log(`首屏 JS（${entryScripts.length} 个文件）gzip: ${kb(entryJs)} / 预算 ${kb(ENTRY_JS_BUDGET)}`)
console.log(`整个 dist gzip: ${kb(total)} / 预算 ${kb(TOTAL_BUDGET)}`)
for (const f of files.filter((f) => f.endsWith('.js') || f.endsWith('.css')).sort()) {
  console.log(`  ${f.slice(DIST.length)}  ${kb(gzipSize(f))}`)
}
if (entryJs > ENTRY_JS_BUDGET || total > TOTAL_BUDGET) {
  console.error('超出体积预算')
  process.exit(1)
}
