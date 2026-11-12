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
import { useLayoutEffect, useRef } from 'react'

/**
 * Density steps tried in order, loosest first. Each step shrinks the font,
 * the truncation caps (`--log-scale` multiplies every `max-w` in the cells)
 * and the horizontal cell padding. The font stops at 11px to stay readable;
 * the last step only squeezes truncatable content further.
 */
export const TABLE_DENSITY_LEVELS = [
  { fontSize: '13px', widthScale: '1', cellPaddingX: '8px' },
  { fontSize: '12px', widthScale: '0.92', cellPaddingX: '8px' },
  { fontSize: '11px', widthScale: '0.85', cellPaddingX: '6px' },
  { fontSize: '11px', widthScale: '0.72', cellPaddingX: '4px' },
] as const

/**
 * Fits the table inside its scroll container by picking the loosest density
 * level that does not overflow horizontally. The level is measured against
 * the real layout, so it adapts to the number of visible columns, the row
 * content and the window width alike. When even the densest level overflows,
 * that level stays applied and the container keeps its horizontal scrollbar.
 *
 * The result is written to the wrapper as `data-density` plus the CSS
 * variables `--log-font`, `--log-scale` and `--log-cell-px`; nothing is
 * stored in React state, so measuring never triggers a re-render.
 *
 * @param enabled   turn fitting off to clear the density from the wrapper
 * @param layoutKey changes whenever the rendered columns or rows change
 */
export function useFitTableDensity<T extends HTMLElement>(
  enabled: boolean,
  layoutKey: string
) {
  const wrapperRef = useRef<T>(null)

  useLayoutEffect(() => {
    const wrapper = wrapperRef.current
    if (!wrapper) return

    const table = wrapper.querySelector<HTMLElement>('[data-slot=table]')
    const scroller = table?.parentElement
    if (!enabled || !table || !scroller) {
      delete wrapper.dataset.density
      wrapper.style.removeProperty('--log-font')
      wrapper.style.removeProperty('--log-scale')
      wrapper.style.removeProperty('--log-cell-px')
      return
    }

    const fit = () => {
      for (const [index, level] of TABLE_DENSITY_LEVELS.entries()) {
        wrapper.dataset.density = String(index)
        wrapper.style.setProperty('--log-font', level.fontSize)
        wrapper.style.setProperty('--log-scale', level.widthScale)
        wrapper.style.setProperty('--log-cell-px', level.cellPaddingX)
        if (scroller.scrollWidth <= scroller.clientWidth) return
      }
    }

    fit()

    // Refit on the next frame instead of inside the observer callback:
    // changing the density there resizes the observed table and would raise
    // "ResizeObserver loop completed with undelivered notifications".
    let frame = 0
    const observer = new ResizeObserver(() => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(fit)
    })
    // The scroller reports window/sidebar resizes; the table reports content
    // that arrives late (IP locations, web fonts) and widens a column.
    observer.observe(scroller)
    observer.observe(table)

    return () => {
      cancelAnimationFrame(frame)
      observer.disconnect()
    }
  }, [enabled, layoutKey])

  return wrapperRef
}
