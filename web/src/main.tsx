// 页面入口：先加载当前语言包再挂载 React，避免先闪英文再变成用户语言。

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from '@/app/App'
import { initialLanguage, loadLanguage } from '@/i18n'
import './index.css'

const container = document.getElementById('root')
if (!container) throw new Error('index.html 里缺少 #root 节点')

const lang = initialLanguage()
void loadLanguage(lang).then(() => {
  createRoot(container).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
})