// 页面入口：加载样式和多语言，把应用挂到 #root 上。

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from '@/app/App'
import '@/i18n'
import './index.css'

const container = document.getElementById('root')
if (!container) throw new Error('index.html 里缺少 #root 节点')
createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
