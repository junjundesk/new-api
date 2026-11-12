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
import { render } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  TABLE_DENSITY_LEVELS,
  useFitTableDensity,
} from '../use-fit-table-density'

const CONTAINER_WIDTH = 1000

// happy-dom has no layout engine, so the table width is simulated: each test
// declares how wide the table would be at every density level.
let tableWidthByLevel: number[] = []

function FittedTable(props: { enabled: boolean; layoutKey: string }) {
  const ref = useFitTableDensity<HTMLDivElement>(props.enabled, props.layoutKey)
  return (
    <div ref={ref} data-testid='wrapper'>
      <div
        ref={(scroller) => {
          if (!scroller) return
          Object.defineProperty(scroller, 'clientWidth', {
            configurable: true,
            get: () => CONTAINER_WIDTH,
          })
          Object.defineProperty(scroller, 'scrollWidth', {
            configurable: true,
            get: () => {
              const wrapper = scroller.parentElement as HTMLElement
              return tableWidthByLevel[Number(wrapper.dataset.density)]
            },
          })
        }}
      >
        <table data-slot='table' />
      </div>
    </div>
  )
}

function renderFitted(enabled = true) {
  const view = render(<FittedTable enabled={enabled} layoutKey='a' />)
  return {
    ...view,
    wrapper: view.getByTestId('wrapper'),
  }
}

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    }
  )
})

afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

describe('useFitTableDensity', () => {
  it('when the table fits at the default size then keeps the loosest level', () => {
    tableWidthByLevel = [900, 800, 700, 600]
    const { wrapper } = renderFitted()

    expect(wrapper.dataset.density).toBe('0')
    expect(wrapper.style.getPropertyValue('--log-font')).toBe(
      TABLE_DENSITY_LEVELS[0].fontSize
    )
  })

  it('when the table overflows then shrinks to the first level that fits', () => {
    tableWidthByLevel = [1400, 1200, 990, 900]
    const { wrapper } = renderFitted()

    expect(wrapper.dataset.density).toBe('2')
    expect(wrapper.style.getPropertyValue('--log-font')).toBe(
      TABLE_DENSITY_LEVELS[2].fontSize
    )
    expect(wrapper.style.getPropertyValue('--log-scale')).toBe(
      TABLE_DENSITY_LEVELS[2].widthScale
    )
  })

  it('when no level fits then stays at the densest level', () => {
    tableWidthByLevel = [2000, 1800, 1600, 1400]
    const { wrapper } = renderFitted()

    expect(wrapper.dataset.density).toBe(
      String(TABLE_DENSITY_LEVELS.length - 1)
    )
  })

  it('when the layout key changes then refits against the new content', () => {
    tableWidthByLevel = [1400, 1200, 990, 900]
    const { wrapper, rerender } = renderFitted()
    expect(wrapper.dataset.density).toBe('2')

    tableWidthByLevel = [900, 800, 700, 600]
    rerender(<FittedTable enabled layoutKey='b' />)

    expect(wrapper.dataset.density).toBe('0')
  })

  it('when fitting is disabled then clears the density from the wrapper', () => {
    tableWidthByLevel = [1400, 1200, 990, 900]
    const { wrapper, rerender } = renderFitted()

    rerender(<FittedTable enabled={false} layoutKey='a' />)

    expect(wrapper.dataset.density).toBeUndefined()
    expect(wrapper.style.getPropertyValue('--log-font')).toBe('')
  })
})
