#!/usr/bin/env node
/**
 * 从 public/favicon.svg 生成桌面应用图标。
 *
 * 运行：在 web/ 目录执行 `node scripts/make-icons.mjs`
 * 依赖：本机已安装的 Google Chrome，以及开发依赖 playwright-core（随 @playwright/test 一起安装，
 *       只用来启动本机 Chrome 渲染图标，不下载浏览器，也不进入前端产物）
 * 输出：../desktop/icon.png
 */

import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { execSync } from 'node:child_process'

/** 动态加载 playwright-core；没装时给出明确提示，而不是一长串模块找不到的报错 */
async function loadChromium() {
  try {
    return (await import('playwright-core')).chromium
  } catch {
    console.error('缺少 playwright-core：请先在 web/ 目录运行 npm install（它是开发依赖）')
    process.exit(1)
  }
}

const chromium = await loadChromium()

const __dir = dirname(fileURLToPath(import.meta.url))
const webRoot = join(__dir, '..')
const svgPath = join(webRoot, 'public/favicon.svg')
const iconPath = join(webRoot, '../desktop/icon.png')

const svgRaw = readFileSync(svgPath, 'utf8')

/**
 * 用 Chrome 将 SVG 画到 1024px 透明画布中央，圆角底板占 824px。
 */
async function rasterize() {
  const size = 1024
  const inner = 824
  const html = `<!DOCTYPE html>
<html><head><meta charset="utf-8"></head>
<body style="margin:0;width:${size}px;height:${size}px;background:transparent;display:flex;align-items:center;justify-content:center;">
<div style="width:${inner}px;height:${inner}px;">${svgRaw}</div>
</body></html>`

  const browser = await chromium.launch({
    channel: 'chrome', // 使用本机已安装的 Chrome，不下载浏览器
    headless: true,
    args: ['--no-sandbox', '--disable-gpu'],
  })
  const page = await browser.newPage({ viewport: { width: size, height: size } })
  await page.setContent(html, { waitUntil: 'load' })
  const png = await page.screenshot({ type: 'png', omitBackground: true })
  await browser.close()
  return png
}

/** 在 macOS 上用自带的 sips 重新编码 PNG 以减小体积；其他系统直接保留 Chrome 输出的原图 */
function shrinkPng(file) {
  if (process.platform !== 'darwin') return
  execSync(`sips -s format png --setProperty formatOptions low "${file}" --out "${file}"`, {
    stdio: 'ignore',
  })
}

async function main() {
  const png = await rasterize()
  writeFileSync(iconPath, png)
  shrinkPng(iconPath)
  console.log(' wrote', iconPath)
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})