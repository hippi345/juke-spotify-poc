import { useEffect, useState } from 'react'
import { AiSparkleIcon } from './AiSparkleIcon'

export type ToastPayload = {
  id: number
  variant: 'success' | 'error'
  title: string
  body?: string
  href?: string
  hrefLabel?: string
  /** Auto-dismiss delay; default 5000ms */
  durationMs?: number
  actionLabel?: string
  onAction?: () => void | Promise<void>
}

type ToastProps = {
  toast: ToastPayload | null
  onDismiss: () => void
}

export function Toast({ toast, onDismiss }: ToastProps) {
  const [actionPending, setActionPending] = useState(false)

  useEffect(() => {
    setActionPending(false)
  }, [toast?.id])

  useEffect(() => {
    if (!toast) return
    const ms = toast.durationMs ?? 5000
    const t = window.setTimeout(onDismiss, ms)
    return () => window.clearTimeout(t)
  }, [toast, onDismiss])

  if (!toast) return null

  const border =
    toast.variant === 'success'
      ? 'border-[#1DB954]/40 bg-[#0d1412]/95'
      : 'border-red-500/40 bg-red-950/90'

  return (
    <div
      className="pointer-events-none fixed left-1/2 top-6 z-[100] flex max-w-sm -translate-x-1/2 flex-col gap-2 px-4"
      role="status"
      aria-live="polite"
    >
      <div
        className={`pointer-events-auto rounded-xl border px-4 py-3 shadow-2xl backdrop-blur-md ${border}`}
      >
        <div className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 flex-1 items-start gap-3">
            {toast.variant === 'success' && (
              <AiSparkleIcon className="h-8 w-8 shrink-0 opacity-95" />
            )}
            <div className="min-w-0">
              <p className="text-sm font-medium text-white">{toast.title}</p>
              {toast.body && <p className="mt-1 text-xs text-zinc-400">{toast.body}</p>}
              {toast.href && (
                <a
                  href={toast.href}
                  target="_blank"
                  rel="noreferrer"
                  className={`mt-2 inline-block text-xs font-medium underline ${
                    toast.variant === 'success' ? 'text-[#1ed760]' : 'text-red-300'
                  }`}
                >
                  {toast.hrefLabel ?? 'Open link'}
                </a>
              )}
              {toast.actionLabel && toast.onAction && (
                <button
                  type="button"
                  disabled={actionPending}
                  onClick={async () => {
                    setActionPending(true)
                    try {
                      await toast.onAction?.()
                    } finally {
                      setActionPending(false)
                    }
                  }}
                  className={`mt-3 w-full rounded-lg px-3 py-2 text-xs font-medium transition disabled:cursor-not-allowed disabled:opacity-60 ${
                    toast.variant === 'success'
                      ? 'border border-[#1DB954]/50 bg-[#1DB954]/20 text-[#1ed760] hover:bg-[#1DB954]/30'
                      : 'border border-white/15 bg-white/10 text-zinc-200 hover:bg-white/15'
                  }`}
                >
                  {actionPending ? 'Starting…' : toast.actionLabel}
                </button>
              )}
            </div>
          </div>
          <button
            type="button"
            onClick={onDismiss}
            className="shrink-0 rounded p-1 text-zinc-500 hover:bg-white/10 hover:text-white"
            aria-label="Dismiss"
          >
            ×
          </button>
        </div>
      </div>
    </div>
  )
}
