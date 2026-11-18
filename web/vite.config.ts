import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'
import { defineConfig } from 'vitest/config'

// 后端地址：开发时前端跑在 5173 端口，接口、MCP、安装说明都转发给本机的 Slate 服务
const backend = process.env.SLATE_BACKEND ?? 'http://127.0.0.1:7070'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': path.resolve(import.meta.dirname, './src') },
  },
  build: {
    // 产物直接放进 Go 程序的嵌入目录，构建时用 -tags embed_web 把它打进可执行文件
    outDir: '../internal/web/spa/dist',
    emptyOutDir: true,
    target: 'es2022',
    // 默认的 500 kB 提示按压缩前算，这里换成按体积预算脚本检查
    chunkSizeWarningLimit: 800,
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    // 机器忙时一个交互较多的测试可能超过默认的 5 秒，放宽到 15 秒，避免误报超时
    testTimeout: 15000,
  },
  server: {
    proxy: {
      '/api': backend,
      '/mcp': backend,
      '/health': backend,
      '/install.md': backend,
      '/skills': backend,
    },
  },
})
