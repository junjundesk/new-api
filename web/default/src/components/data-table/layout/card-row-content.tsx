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
import type { Cell, Row } from '@tanstack/react-table'

import { getCellLabel, renderCellContent } from './card-cell-utils'
import { DataTableCardDetails, DataTableCardField } from './card-field'

type CardRole = 'title' | 'badge' | 'primary' | 'secondary' | 'hidden'

function getCardRole<TData>(cell: Cell<TData, unknown>): CardRole {
  const meta = cell.column.columnDef.meta
  if (meta?.cardRole) return meta.cardRole
  return 'primary'
}

function orderCardCells<TData>(
  cells: Cell<TData, unknown>[]
): Cell<TData, unknown>[] {
  return [...cells].sort((a, b) => {
    const aOrder = a.column.columnDef.meta?.cardOrder
    const bOrder = b.column.columnDef.meta?.cardOrder

    if (aOrder == null && bOrder == null) return 0
    if (aOrder == null) return 1
    if (bOrder == null) return -1
    return aOrder - bOrder
  })
}

function CardFields<TData>({ cells }: { cells: Cell<TData, unknown>[] }) {
  return cells.map((cell) => {
    const meta = cell.column.columnDef.meta
    return (
      <DataTableCardField
        key={cell.id}
        label={getCellLabel(cell)}
        contentMode={meta?.contentMode}
        span={meta?.cardSpan}
      >
        {renderCellContent(cell)}
      </DataTableCardField>
    )
  })
}

/**
 * Shared row content for both the mobile list and optional desktop card grid.
 * Primary values never clip silently; lower-priority values remain available
 * through the shared progressive details disclosure.
 */
export function CardRowContent<TData>(props: {
  row: Row<TData>
  compact: boolean
}) {
  const cells = props.row
    .getVisibleCells()
    .filter((cell) => cell.column.id !== 'select')
  const titleCell = cells.find((cell) => getCardRole(cell) === 'title')
  const badgeCell = cells.find((cell) => getCardRole(cell) === 'badge')
  const actionsCell = cells.find((cell) => cell.column.id === 'actions')
  const fieldCells = orderCardCells(
    cells.filter(
      (cell) =>
        cell !== titleCell &&
        cell !== badgeCell &&
        cell !== actionsCell &&
        getCardRole(cell) === 'primary'
    )
  )
  const secondaryCells = orderCardCells(
    cells.filter(
      (cell) =>
        cell !== titleCell &&
        cell !== badgeCell &&
        cell !== actionsCell &&
        getCardRole(cell) === 'secondary'
    )
  )

  return (
    <>
      {props.compact && (titleCell || badgeCell) && (
        <div className='flex min-w-0 items-start justify-between gap-3'>
          <div className='min-w-0 flex-1 text-sm font-medium [overflow-wrap:anywhere] break-words whitespace-normal [&_.truncate]:overflow-visible [&_.truncate]:text-clip [&_.truncate]:whitespace-normal [&_[data-slot=status-badge-label]]:whitespace-normal [&_[data-slot=status-badge]]:h-auto [&_[data-slot=status-badge]]:max-w-full'>
            {titleCell ? renderCellContent(titleCell) : null}
          </div>
          {badgeCell && (
            <DataTableCardField
              contentMode={badgeCell.column.columnDef.meta?.contentMode}
              className='max-w-1/2 shrink'
              valueClassName='flex justify-end text-right'
            >
              {renderCellContent(badgeCell)}
            </DataTableCardField>
          )}
        </div>
      )}

      {fieldCells.length > 0 && (
        <div
          className={
            props.compact
              ? 'mt-2 grid grid-cols-2 gap-x-3 gap-y-2'
              : 'grid grid-cols-2 gap-x-3 gap-y-2'
          }
        >
          <CardFields cells={fieldCells} />
        </div>
      )}

      {secondaryCells.length > 0 && (
        <DataTableCardDetails count={secondaryCells.length}>
          <CardFields cells={secondaryCells} />
        </DataTableCardDetails>
      )}

      {actionsCell && (
        <div className='mt-2 -mb-0.5 flex justify-end border-t pt-2'>
          {renderCellContent(actionsCell)}
        </div>
      )}
    </>
  )
}
