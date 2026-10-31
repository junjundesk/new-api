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
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { cleanup, render } from '@testing-library/react'
import { useMemo } from 'react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { DataTableColumnHeader } from '../column-header'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

interface RowData {
  amount: number
}

function Harness(props: { align?: 'start' | 'end'; sortable?: boolean }) {
  const columns = useMemo(
    () => [
      {
        id: 'amount',
        accessorKey: 'amount' as const,
        header: 'Amount',
        enableSorting: props.sortable ?? true,
      },
    ],
    [props.sortable]
  )
  const table = useReactTable<RowData>({
    data: [{ amount: 1 }],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <DataTableColumnHeader
      column={table.getAllColumns()[0]}
      title='Amount'
      align={props.align}
    />
  )
}

afterEach(() => {
  cleanup()
})

describe('DataTableColumnHeader alignment', () => {
  test('right-aligns a sortable header so it lines up with numeric cells', () => {
    const { container } = render(<Harness align='end' />)
    expect(container.firstElementChild?.className).toContain('justify-end')
  })

  test('right-aligns a non-sortable header through text alignment', () => {
    const { container } = render(<Harness align='end' sortable={false} />)
    const header = container.firstElementChild
    expect(header?.className).toContain('text-right')
    // A non-sortable header renders a plain div, so the flex alignment class
    // has no effect and must not be relied on.
    expect(header?.className).not.toContain('justify-end')
  })

  test('keeps the default left alignment when align is omitted', () => {
    const { container } = render(<Harness />)
    const header = container.firstElementChild
    expect(header?.className).not.toContain('justify-end')
    expect(header?.className).not.toContain('text-right')
  })
})
