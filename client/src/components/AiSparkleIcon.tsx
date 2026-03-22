import { useId } from 'react'

type AiSparkleIconProps = {
  className?: string
  /** Larger mark for section headers */
  variant?: 'default' | 'lg'
}

/** Sparkle / star cluster mark (AI-style) — gradient, no external assets */
export function AiSparkleIcon({ className, variant = 'default' }: AiSparkleIconProps) {
  const id = useId().replace(/:/g, '')
  const gradId = `ai-sparkle-grad-${id}`
  const size = variant === 'lg' ? 'h-11 w-11' : 'h-6 w-6'
  const g = `url(#${gradId})`

  return (
    <svg
      className={className ?? size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <defs>
        <linearGradient id={gradId} x1="4" y1="2" x2="20" y2="22" gradientUnits="userSpaceOnUse">
          <stop stopColor="#f5d0fe" />
          <stop offset="0.4" stopColor="#a78bfa" />
          <stop offset="1" stopColor="#6d28d9" />
        </linearGradient>
      </defs>
      {/* Main gem / star diamond */}
      <path fill={g} d="M12 2.25L20 12l-8 9.75L4 12 12 2.25z" />
      {/* Sparkles */}
      <circle cx="18.65" cy="4.85" r="1.2" fill={g} opacity={0.95} />
      <circle cx="5.1" cy="6.5" r="0.8" fill={g} opacity={0.88} />
      <circle cx="19.4" cy="14.35" r="0.7" fill={g} opacity={0.85} />
      <circle cx="3.35" cy="14" r="0.55" fill={g} opacity={0.75} />
    </svg>
  )
}
