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
import {
  flexRender,
  type Cell,
  type Row,
  type Table,
} from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import {
  DataTableCardDetails,
  DataTableCardField,
  MobileCardList,
} from '@/components/data-table'
import { cn } from '@/lib/utils'

import { LOG_TYPE_ENUM } from '../constants'

const logTypeRowTint: Record<number, string> = {
  [LOG_TYPE_ENUM.ERROR]: 'bg-destructive/5 border-destructive/20',
  [LOG_TYPE_ENUM.REFUND]: 'bg-info/5 border-info/20',
}

interface UsageLogsMobileListProps<TData> {
  table: Table<TData>
  isLoading?: boolean
  emptyTitle?: string
  emptyDescription?: string
  getRowClassName?: (row: Row<TData>) => string | undefined
}

type CardRole = 'title' | 'badge' | 'primary' | 'secondary' | 'hidden'

function getCardRole<TData>(cell: Cell<TData, unknown>): CardRole {
  return cell.column.columnDef.meta?.cardRole ?? 'primary'
}

function getCardLabel<TData>(cell: Cell<TData, unknown>): string {
  const metaLabel = cell.column.columnDef.meta?.label
  if (metaLabel) return metaLabel

  const header = cell.column.columnDef.header
  return typeof header === 'string' ? header : cell.column.id
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

function CardCellField<TData>(props: {
  cell: Cell<TData, unknown>
  hideLabel?: boolean
  className?: string
  valueClassName?: string
}) {
  const meta = props.cell.column.columnDef.meta

  return (
    <DataTableCardField
      label={props.hideLabel ? undefined : getCardLabel(props.cell)}
      contentMode={meta?.contentMode}
      span={meta?.cardSpan}
      className={props.className}
      valueClassName={props.valueClassName}
    >
      {flexRender(props.cell.column.columnDef.cell, props.cell.getContext())}
    </DataTableCardField>
  )
}

function UsageLogCard<TData>(props: { cells: Cell<TData, unknown>[] }) {
  const titleCell = props.cells.find((cell) => getCardRole(cell) === 'title')
  const badgeCell = props.cells.find((cell) => getCardRole(cell) === 'badge')
  const primaryCells = orderCardCells(
    props.cells.filter((cell) => getCardRole(cell) === 'primary')
  )
  const secondaryCells = orderCardCells(
    props.cells.filter((cell) => getCardRole(cell) === 'secondary')
  )

  return (
    <>
      {(titleCell || badgeCell) && (
        <div className='flex min-w-0 items-start justify-between gap-3'>
          {titleCell && (
            <CardCellField
              cell={titleCell}
              hideLabel
              className='min-w-0 flex-1'
              valueClassName='font-medium'
            />
          )}
          {badgeCell && (
            <CardCellField
              cell={badgeCell}
              hideLabel
              className='max-w-1/2 shrink text-right'
              valueClassName='flex justify-end text-right tabular-nums'
            />
          )}
        </div>
      )}

      {primaryCells.length > 0 && (
        <div className='mt-2 grid grid-cols-2 gap-x-3 gap-y-2'>
          {primaryCells.map((cell) => (
            <CardCellField key={cell.id} cell={cell} />
          ))}
        </div>
      )}

      {secondaryCells.length > 0 && (
        <DataTableCardDetails count={secondaryCells.length}>
          {secondaryCells.map((cell) => (
            <CardCellField key={cell.id} cell={cell} />
          ))}
        </DataTableCardDetails>
      )}
    </>
  )
}

export function UsageLogsMobileList<TData>({
  table,
  isLoading = false,
  emptyTitle,
  emptyDescription,
  getRowClassName,
}: UsageLogsMobileListProps<TData>) {
  const { t } = useTranslation()

  const resolvedEmptyTitle = emptyTitle ?? t('No Logs Found')
  const resolvedEmptyDescription =
    emptyDescription ??
    t('No usage logs available. Logs will appear here once API calls are made.')

  return (
    <MobileCardList
      table={table}
      isLoading={isLoading}
      emptyTitle={resolvedEmptyTitle}
      emptyDescription={resolvedEmptyDescription}
      renderCard={(row) => <UsageLogCard cells={row.getVisibleCells()} />}
      getRowClassName={(row) => {
        const logType = (row.original as Record<string, unknown>).type as
          | number
          | undefined
        const tintClass = logType != null ? (logTypeRowTint[logType] ?? '') : ''
        return cn(
          'border-l-2 border-l-transparent transition-colors',
          tintClass,
          getRowClassName?.(row)
        )
      }}
    />
  )
}
