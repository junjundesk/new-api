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
import { flexRender, type Row } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import {
  DataTableCardDetails,
  DataTableCardField,
} from '@/components/data-table'

import { isTagAggregateRow } from '../lib'
import type { Channel } from '../types'
import { ChannelRowActionsLayoutContext } from './channel-row-actions-context'

/**
 * Bespoke channel card for the card view. Reuses every column's existing cell
 * renderer via `flexRender`, so the table's information and interactions are
 * preserved: row selection, provider/multi-key/IO.NET type badge, id,
 * name/remark + warning icons, status (with tooltips), groups, inline
 * priority/weight spinners, balance refresh, response/test times, tag
 * expand-collapse, models, and the per-row (or per-tag) actions menu.
 */
function ChannelCardComponent({
  row,
  isSelected,
}: {
  row: Row<Channel>
  isSelected: boolean
}) {
  const { t } = useTranslation()
  const isTagRow = isTagAggregateRow(row.original)
  const cells = row.getVisibleCells()
  const visibleColumnIds = new Set(cells.map((cell) => cell.column.id))

  const renderCell = (id: string) => {
    const cell = cells.find((candidate) => candidate.column.id === id)
    if (!cell || !cell.column.columnDef.cell) {
      return null
    }
    return flexRender(cell.column.columnDef.cell, cell.getContext())
  }

  const selectCell = renderCell('select')
  const typeCell = renderCell('type')
  const idCell = renderCell('id')
  const nameCell = renderCell('name')
  const statusCell = renderCell('status')
  const actionsCell = renderCell('actions')
  const priorityCell = renderCell('priority')
  const weightCell = renderCell('weight')
  const balanceCell = renderCell('balance')
  const modelsCell = renderCell('models')
  const groupsCell = renderCell('group')
  const tagCell = renderCell('tag')
  const responseCell = renderCell('response_time')
  const testCell = renderCell('test_time')

  const emptyValue = <span className='text-muted-foreground'>-</span>
  const detailsCount = [
    visibleColumnIds.has('group'),
    !isTagRow && visibleColumnIds.has('tag'),
    visibleColumnIds.has('priority'),
    visibleColumnIds.has('weight'),
  ].filter(Boolean).length

  return (
    <ChannelRowActionsLayoutContext.Provider value='card'>
      <div
        data-state={isSelected ? 'selected' : undefined}
        className='flex flex-col gap-3'
      >
        {/* Provider identity, status, selection, and every row action remain
            immediately available. The wrapping layout avoids mobile clipping. */}
        <div className='flex flex-wrap items-start justify-between gap-2'>
          <div className='flex min-w-0 flex-1 items-center gap-2'>
            {!isTagRow && selectCell && (
              <span className='shrink-0'>{selectCell}</span>
            )}
            {visibleColumnIds.has('type') && (
              <div className='min-w-0 flex-1'>{typeCell}</div>
            )}
          </div>
          <div className='flex flex-wrap items-center justify-end gap-1.5'>
            {visibleColumnIds.has('status') && statusCell}
            {actionsCell}
          </div>
        </div>

        <div className='grid grid-cols-2 gap-x-3 gap-y-2'>
          {visibleColumnIds.has('name') && (
            <DataTableCardField
              label={isTagRow ? t('Tag') : t('Name')}
              span={2}
              contentMode='wrap'
            >
              {nameCell ?? emptyValue}
            </DataTableCardField>
          )}
          {!isTagRow && visibleColumnIds.has('id') && (
            <DataTableCardField label={t('ID')} contentMode='full'>
              {idCell ?? emptyValue}
            </DataTableCardField>
          )}
          {visibleColumnIds.has('balance') && (
            <DataTableCardField
              label={t('Used / Remaining')}
              span={2}
              contentMode='full'
            >
              {balanceCell ?? emptyValue}
            </DataTableCardField>
          )}
          {!isTagRow && visibleColumnIds.has('models') && (
            <DataTableCardField
              label={t('Models')}
              span={2}
              contentMode='summary'
            >
              {modelsCell ?? emptyValue}
            </DataTableCardField>
          )}
          {visibleColumnIds.has('response_time') && (
            <DataTableCardField label={t('Response')} contentMode='full'>
              {responseCell ?? emptyValue}
            </DataTableCardField>
          )}
          {!isTagRow && visibleColumnIds.has('test_time') && (
            <DataTableCardField label={t('Last Tested')} contentMode='full'>
              {testCell ?? emptyValue}
            </DataTableCardField>
          )}
        </div>

        {detailsCount > 0 && (
          <DataTableCardDetails count={detailsCount}>
            {visibleColumnIds.has('group') && (
              <DataTableCardField
                label={t('Groups')}
                span={2}
                contentMode='summary'
              >
                {groupsCell ?? emptyValue}
              </DataTableCardField>
            )}
            {!isTagRow && visibleColumnIds.has('tag') && (
              <DataTableCardField label={t('Tag')} span={2} contentMode='wrap'>
                {tagCell ?? emptyValue}
              </DataTableCardField>
            )}
            {visibleColumnIds.has('priority') && (
              <DataTableCardField label={t('Priority')} contentMode='full'>
                {priorityCell ?? emptyValue}
              </DataTableCardField>
            )}
            {visibleColumnIds.has('weight') && (
              <DataTableCardField label={t('Weight')} contentMode='full'>
                {weightCell ?? emptyValue}
              </DataTableCardField>
            )}
          </DataTableCardDetails>
        )}
      </div>
    </ChannelRowActionsLayoutContext.Provider>
  )
}

export const ChannelCard = ChannelCardComponent
