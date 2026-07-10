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
  BadgeListCellDisplayContext,
  DataTableCardField,
  DataTableCardRow,
} from '@/components/data-table'

import { isTagAggregateRow } from '../lib'
import type { Channel } from '../types'
import { ChannelRowActionsLayoutContext } from './channel-row-actions-context'

/**
 * Bespoke channel card for the card view. Reuses every column's existing cell
 * renderer via `flexRender`, so the table's information and interactions are
 * preserved. All fields are always visible — no "More" disclosure.
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

  const showId = !isTagRow && visibleColumnIds.has('id')
  const showTag = !isTagRow && visibleColumnIds.has('tag')
  const showModels = !isTagRow && visibleColumnIds.has('models')
  const showTestTime = !isTagRow && visibleColumnIds.has('test_time')
  const hasStatRows =
    showId ||
    showTag ||
    showTestTime ||
    visibleColumnIds.has('balance') ||
    visibleColumnIds.has('response_time') ||
    visibleColumnIds.has('priority') ||
    visibleColumnIds.has('weight')
  const hasBadgeSections = showModels || visibleColumnIds.has('group')

  return (
    <ChannelRowActionsLayoutContext.Provider value='card'>
      <BadgeListCellDisplayContext.Provider value='full'>
        <div
          data-state={isSelected ? 'selected' : undefined}
          className='flex h-full min-w-0 flex-col'
        >
          <div className='flex min-w-0 items-start gap-2.5'>
            {!isTagRow && selectCell && (
              <span className='mt-0.5 shrink-0'>{selectCell}</span>
            )}

            <div className='min-w-0 flex-1'>
              {visibleColumnIds.has('name') && (
                <div className='min-w-0 text-[15px] leading-tight font-semibold break-words'>
                  {nameCell}
                </div>
              )}
              {visibleColumnIds.has('type') && (
                <div className='mt-1.5 min-w-0'>{typeCell}</div>
              )}
            </div>

            <div className='flex shrink-0 items-center gap-1'>
              {visibleColumnIds.has('status') && statusCell}
              {actionsCell}
            </div>
          </div>

          {hasStatRows && (
            <div className='mt-3 space-y-0.5 border-t pt-3'>
              {showId && (
                <DataTableCardRow label={t('ID')} contentMode='full'>
                  {idCell}
                </DataTableCardRow>
              )}
              {visibleColumnIds.has('balance') && (
                <DataTableCardRow
                  label={t('Used / Remaining')}
                  contentMode='full'
                >
                  {balanceCell}
                </DataTableCardRow>
              )}
              {visibleColumnIds.has('response_time') && (
                <DataTableCardRow label={t('Response')} contentMode='full'>
                  {responseCell}
                </DataTableCardRow>
              )}
              {showTestTime && (
                <DataTableCardRow label={t('Last Tested')} contentMode='full'>
                  {testCell}
                </DataTableCardRow>
              )}
              {visibleColumnIds.has('priority') && (
                <DataTableCardRow label={t('Priority')} contentMode='full'>
                  {priorityCell}
                </DataTableCardRow>
              )}
              {visibleColumnIds.has('weight') && (
                <DataTableCardRow label={t('Weight')} contentMode='full'>
                  {weightCell}
                </DataTableCardRow>
              )}
              {showTag && (
                <DataTableCardRow label={t('Tag')} contentMode='wrap'>
                  {tagCell}
                </DataTableCardRow>
              )}
            </div>
          )}

          {hasBadgeSections && (
            <div className='mt-3 space-y-3 border-t pt-3'>
              {visibleColumnIds.has('group') && (
                <DataTableCardField label={t('Groups')} contentMode='full'>
                  {groupsCell ?? (
                    <span className='text-muted-foreground'>-</span>
                  )}
                </DataTableCardField>
              )}
              {showModels && (
                <DataTableCardField label={t('Models')} contentMode='full'>
                  {modelsCell ?? (
                    <span className='text-muted-foreground'>-</span>
                  )}
                </DataTableCardField>
              )}
            </div>
          )}
        </div>
      </BadgeListCellDisplayContext.Provider>
    </ChannelRowActionsLayoutContext.Provider>
  )
}

export const ChannelCard = ChannelCardComponent
