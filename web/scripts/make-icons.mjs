#!/usr/bin/env node
/**
 * 从 public/favicon.svg 生成 PWA / Apple 用的 PNG 图标。
 *
 * 运行：在 web/ 目录执行 `node scripts/make-icons.mjs`
 * 依赖：本机已安装的 Google Chrome，以及开发依赖 playwright-core（随 @playwright/test 一起安装，
 *       只用来启动本机 Chrome 渲染图标，不下载浏览器，也不进入前端产物）
 * 输出：public/icons/*.png、public/apple-touch-icon.png
 */

import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
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
const iconsDir = join(webRoot, 'public/icons')

/** maskable 图标四周至少留 20% 安全区，内容约占画布的 60% */
const MASKABLE_CONTENT_RATIO = 0.6

const svgRaw = readFileSync(svgPath, 'utf8')

/**
 * 用 Chrome 把 SVG 画进指定尺寸的 PNG；可选背景色与内边距（maskable 用）。
 */
async function rasterize(size, { padding = 0, bg = '#1d1d1f' } = {}) {
  const inner = size - padding * 2
  const html = `<!DOCTYPE html>
<html><head><meta charset="utf-8"></head>
<body style="margin:0;width:${size}px;height:${size}px;background:${bg};display:flex;align-items:center;justify-content:center;">
<div style="width:${inner}px;height:${inner}px;">${svgRaw}</div>
</body></html>`

  const browser = await chromium.launch({
    channel: 'chrome', // 使用本机已安装的 Chrome，不下载浏览器
    headless: true,
    args: ['--no-sandbox', '--disable-gpu'],
  })
  const page = await browser.newPage({ viewport: { width: size, height: size } })
  await page.setContent(html, { waitUntil: 'load' })
  const png = await page.screenshot({ type: 'png', omitBackground: false })
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
  mkdirSync(iconsDir, { recursive: true })

  const jobs = [
    { path: join(iconsDir, 'icon-192.png'), size: 192, padding: 0, bg: '#1d1d1f' },
    { path: join(iconsDir, 'icon-512.png'), size: 512, padding: 0, bg: '#1d1d1f' },
    {
      path: join(iconsDir, 'icon-maskable-512.png'),
      size: 512,
      padding: Math.round(512 * (1 - MASKABLE_CONTENT_RATIO) / 2),
      bg: '#1d1d1f',
    },
    { path: join(webRoot, 'public/apple-touch-icon.png'), size: 180, padding: 0, bg: '#1d1d1f' },
  ]

  for (const job of jobs) {
    const buf = await rasterize(job.size, { padding: job.padding, bg: job.bg })
    writeFileSync(job.path, buf)
    shrinkPng(job.path)
    console.log(' wrote', job.path)
  }
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})