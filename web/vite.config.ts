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
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
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
