// 在页面渲染之前就确定深浅主题，避免刷新时先白一下再变黑。
// 这里的规则要和 src/lib/theme.tsx 保持一致：存储键是 slate.theme，取值 light、dark、system（缺省 system）。
// 做成外链脚本而不是内联脚本，是为了让服务端的内容安全策略不用放开 unsafe-inline。
;(function () {
  // macOS WebView 在首屏就为原生控制按钮留出空间。
  if (navigator.userAgent.endsWith(' SlateDesktop')) document.documentElement.dataset.slateMac = ''
  var dark = false
  try {
    var pref = localStorage.getItem('slate.theme') || 'system'
    dark = pref === 'dark' || (pref === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  } catch {
    // 读不到偏好（比如浏览器禁用了本地存储）就用浅色，页面照常使用
    dark = false
  }
  document.documentElement.classList.toggle('dark', dark)
})()
