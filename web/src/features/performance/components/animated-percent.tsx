/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useEffect, useState } from 'react'

import { formatUptimePct } from '@/features/performance-metrics/lib/format'

const ANIMATION_DURATION_MS = 700

function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined') return true
  if (typeof window.matchMedia !== 'function') return true
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/** Availability figure that counts up to its value in 700ms. */
export function AnimatedPercent(props: { value: number }) {
  const [display, setDisplay] = useState(() =>
    prefersReducedMotion() ? props.value : 0
  )

  useEffect(() => {
    if (prefersReducedMotion()) {
      setDisplay(props.value)
      return
    }
    let start: number | null = null
    let frame = 0
    const step = (timestamp: number) => {
      if (start === null) start = timestamp
      const progress = Math.min(1, (timestamp - start) / ANIMATION_DURATION_MS)
      setDisplay(props.value * (1 - Math.pow(1 - progress, 5)))
      if (progress < 1) frame = window.requestAnimationFrame(step)
    }
    frame = window.requestAnimationFrame(step)
    return () => window.cancelAnimationFrame(frame)
  }, [props.value])

  return <>{formatUptimePct(display)}</>
}
