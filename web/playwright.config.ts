// Playwright 端到端冒烟测试的配置：用例在真实的 Chrome 里操作真实的 Slate 服务。
// 服务由 e2e/global-setup.ts 在每次运行前编译并启动（临时目录、独立端口、全新的数据库）。

import { defineConfig } from '@playwright/test'
import { BASE_URL } from './e2e/env'

export default defineConfig({
  testDir: './e2e',
  // 用 .e2e.ts 后缀，避免被 vitest 当成单元测试
  testMatch: '**/*.e2e.ts',
  globalSetup: './e2e/global-setup.ts',
  // 用例互相独立（各自新建项目），串行跑更稳；失败就是失败，不自动重试
  workers: 1,
  fullyParallel: false,
  retries: 0,
  timeout: 30_000,
  expect: { timeout: 7_000 },
  reporter: [['list']],
  outputDir: './e2e/.results',
  use: {
    baseURL: BASE_URL,
    // 用本机装的 Google Chrome，不用另外下载浏览器
    channel: 'chrome',
    locale: 'en-US',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
})
