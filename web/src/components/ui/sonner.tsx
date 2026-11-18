// 自研 Toaster：右下角堆叠提示，导出名 Toaster 供 App 与测试挂载。

import { useEffect, useRef, useSyncExternalStore, useState, type ReactNode } from "react"
import {
  CircleCheckIcon,
  InfoIcon,
  OctagonXIcon,
  TriangleAlertIcon,
  XIcon,
} from "lucide-react"
import { cn } from "cn"
import { useTranslation } from "@/i18n/react"
import {
  dismissToast,
  getToastsSnapshot,
  subscribeToasts,
  type ToastRecord,
  type ToastVariant,
} from "@/lib/toast-store"

const icons: Record<ToastVariant, ReactNode> = {
  default: <InfoIcon className="size-4" aria-hidden />,
  success: <CircleCheckIcon className="size-4" aria-hidden />,
  info: <InfoIcon className="size-4" aria-hidden />,
  warning: <TriangleAlertIcon className="size-4" aria-hidden />,
  error: <OctagonXIcon className="size-4" aria-hidden />,
}

/** 单条提示：自己的倒计时，悬停或聚焦时暂停 */
function ToastItem({ item, onClose }: { item: ToastRecord; onClose: (id: string | number) => void }) {
  const { t } = useTranslation()
  const [paused, setPaused] = useState(false)
  const remainingRef = useRef(item.duration)
  const lastTickRef = useRef(Date.now())

  useEffect(() => {
    remainingRef.current = item.duration
    lastTickRef.current = Date.now()
  }, [item.id, item.duration, item.title, item.description])

  useEffect(() => {
  // 尊重减少动画偏好：不自动消失，只能手动关
    const reduced =
      typeof window.matchMedia === "function" && window.matchMedia("(prefers-reduced-motion: reduce)").matches
    if (reduced) return

    let timer: ReturnType<typeof setTimeout> | undefined
    const schedule = () => {
      if (paused) return
      timer = setTimeout(() => onClose(item.id), remainingRef.current)
    }
    schedule()
    return () => {
      if (timer) clearTimeout(timer)
    }
  }, [item.id, onClose, paused])

  const pause = () => {
    if (paused) return
    setPaused(true)
    remainingRef.current -= Date.now() - lastTickRef.current
  }
  const resume = () => {
    if (!paused) return
    setPaused(false)
    lastTickRef.current = Date.now()
  }

  const isError = item.variant === "error"

  return (
    <li
      role={isError ? "alert" : "status"}
      className={cn(
        "pointer-events-auto flex w-full max-w-sm gap-3 border bg-popover p-4 text-popover-foreground shadow-md",
        "data-[state=open]:animate-in data-[state=closed]:animate-out",
        isError && "border-destructive/40",
      )}
      onMouseEnter={pause}
      onMouseLeave={resume}
      onFocus={pause}
      onBlur={resume}
    >
      <span className="mt-0.5 shrink-0 text-muted-foreground">{icons[item.variant]}</span>
      <div className="min-w-0 flex-1 space-y-1">
        <p className="text-sm font-medium leading-snug">{item.title}</p>
        {item.description ? (
          <p className="whitespace-pre-wrap text-sm text-muted-foreground">{item.description}</p>
        ) : null}
      </div>
      <button
        type="button"
        className="shrink-0 rounded-md p-1 text-muted-foreground hover:text-foreground"
        onClick={() => onClose(item.id)}
        aria-label={t("common.close")}
      >
        <XIcon className="size-4" />
      </button>
    </li>
  )
}

/** 可选属性；closeButton 保留只是为了调用方的写法不用改，关闭钮由每条提示自带 */
type ToasterProps = { closeButton?: boolean }

/** 全局提示容器：供 App 根节点挂载 */
function Toaster(_props: ToasterProps = {}) {
  const items = useSyncExternalStore(subscribeToasts, getToastsSnapshot, getToastsSnapshot)

  return (
    <div
      className="pointer-events-none fixed inset-x-0 bottom-0 z-50 flex justify-center p-4 sm:inset-x-auto sm:right-0 sm:left-auto sm:justify-end"
      aria-live="polite"
    >
      <ul className="flex w-full max-w-sm flex-col gap-2 sm:w-auto">
        {items.map((item) => (
          <ToastItem key={String(item.id)} item={item} onClose={dismissToast} />
        ))}
      </ul>
    </div>
  )
}

export { Toaster }