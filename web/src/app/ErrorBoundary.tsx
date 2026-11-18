// 页面渲染出错时的兜底：不让整个应用白屏，给出提示和刷新按钮，并把错误记到控制台方便排查。

import { Component, type ErrorInfo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'

/** 兜底界面：错误标题、具体信息（开发者看的）、重新加载按钮 */
function ErrorScreen({ error }: { error: Error }) {
  const { t } = useTranslation()
  return (
    <div role="alert" className="flex min-h-dvh flex-col items-center justify-center gap-3 p-8 text-center">
      <h1 className="text-lg font-semibold">{t('errors.title')}</h1>
      <p className="max-w-md font-mono text-xs break-words text-muted-foreground">{error.message}</p>
      <Button onClick={() => window.location.reload()}>{t('common.reload')}</Button>
    </div>
  )
}

interface BoundaryState {
  error: Error | null
}

/** 错误边界：捕获子组件渲染时抛出的错误；React 只支持用类来写它 */
export class ErrorBoundary extends Component<{ children: ReactNode }, BoundaryState> {
  state: BoundaryState = { error: null }

  /** 子组件出错时把错误存进状态，下一次渲染就显示兜底界面 */
  static getDerivedStateFromError(error: Error): BoundaryState {
    return { error }
  }

  /** 把错误和组件栈记到控制台，线上排查靠它 */
  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error('slate: render error', error, info.componentStack)
  }

  /** 没有错误时原样渲染子组件，有错误时显示兜底界面 */
  render(): ReactNode {
    return this.state.error ? <ErrorScreen error={this.state.error} /> : this.props.children
  }
}
