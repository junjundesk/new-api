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
import { StatusBadge } from '@/components/status-badge'
import { formatQuota } from '@/lib/format'

import type { ApiKey } from '../types'

function renderApiKeyCell(row: Row<ApiKey>, columnId: string) {
  const cell = row
    .getVisibleCells()
    .find((candidate) => candidate.column.id === columnId)
  if (!cell) return null
  return flexRender(cell.column.columnDef.cell, cell.getContext())
}

function ApiKeyModels(props: { apiKey: ApiKey }) {
  const { t } = useTranslation()

  if (!props.apiKey.model_limits_enabled || !props.apiKey.model_limits) {
    return <StatusBadge variant='neutral'>{t('Unlimited')}</StatusBadge>
  }

  const models = props.apiKey.model_limits
    .split(',')
    .map((model) => model.trim())
    .filter(Boolean)

  return (
    <span className='font-mono text-xs whitespace-pre-wrap'>
      {models.join(', ')}
    </span>
  )
}

function ApiKeyIpRestrictions(props: { apiKey: ApiKey }) {
  const { t } = useTranslation()
  const allowIps = props.apiKey.allow_ips?.trim()

  if (!allowIps) {
    return <StatusBadge variant='neutral'>{t('No restriction')}</StatusBadge>
  }

  const ips = allowIps
    .split('\n')
    .map((ip) => ip.trim())
    .filter(Boolean)

  return (
    <span className='font-mono text-xs whitespace-pre-wrap'>
      {ips.join('\n')}
    </span>
  )
}

export function ApiKeyCard(props: { row: Row<ApiKey> }) {
  const { t } = useTranslation()
  const apiKey = props.row.original
  const totalQuota = apiKey.used_quota + apiKey.remain_quota
  const visibleColumnIds = new Set(
    props.row.getVisibleCells().map((cell) => cell.column.id)
  )
  const detailsCount = [
    'status',
    'group',
    'model_limits',
    'allow_ips',
    'created_time',
    'accessed_time',
    'expired_time',
    'actions',
  ].filter((columnId) => visibleColumnIds.has(columnId)).length

  return (
    <>
      <div className='grid grid-cols-2 gap-x-3 gap-y-2'>
        {visibleColumnIds.has('name') && (
          <DataTableCardField
            label={t('Name')}
            span={2}
            contentMode='wrap'
            valueClassName='font-medium'
          >
            {renderApiKeyCell(props.row, 'name')}
          </DataTableCardField>
        )}
        {visibleColumnIds.has('key') && (
          <DataTableCardField label={t('API Key')} span={2} contentMode='full'>
            {renderApiKeyCell(props.row, 'key')}
          </DataTableCardField>
        )}
        {visibleColumnIds.has('quota') && (
          <DataTableCardField label={t('Quota')} span={2} contentMode='full'>
            {apiKey.unlimited_quota ? (
              <StatusBadge variant='neutral'>{t('Unlimited')}</StatusBadge>
            ) : (
              <span className='font-medium tabular-nums'>
                {formatQuota(apiKey.remain_quota)}
                <span className='text-muted-foreground font-normal'>
                  {' / '}
                  {formatQuota(totalQuota)}
                </span>
              </span>
            )}
          </DataTableCardField>
        )}
      </div>

      {detailsCount > 0 && (
        <DataTableCardDetails count={detailsCount}>
          {visibleColumnIds.has('status') && (
            <DataTableCardField label={t('Status')} contentMode='full'>
              {renderApiKeyCell(props.row, 'status')}
            </DataTableCardField>
          )}
          {visibleColumnIds.has('group') && (
            <DataTableCardField label={t('Group')} contentMode='full'>
              {renderApiKeyCell(props.row, 'group')}
            </DataTableCardField>
          )}
          {visibleColumnIds.has('model_limits') && (
            <DataTableCardField label={t('Models')} span={2} contentMode='full'>
              <ApiKeyModels apiKey={apiKey} />
            </DataTableCardField>
          )}
          {visibleColumnIds.has('allow_ips') && (
            <DataTableCardField
              label={t('IP Restriction')}
              span={2}
              contentMode='full'
            >
              <ApiKeyIpRestrictions apiKey={apiKey} />
            </DataTableCardField>
          )}
          {visibleColumnIds.has('created_time') && (
            <DataTableCardField label={t('Created')} contentMode='full'>
              {renderApiKeyCell(props.row, 'created_time')}
            </DataTableCardField>
          )}
          {visibleColumnIds.has('accessed_time') && (
            <DataTableCardField label={t('Last Used')} contentMode='full'>
              {renderApiKeyCell(props.row, 'accessed_time')}
            </DataTableCardField>
          )}
          {visibleColumnIds.has('expired_time') && (
            <DataTableCardField
              label={t('Expires')}
              span={2}
              contentMode='full'
            >
              {renderApiKeyCell(props.row, 'expired_time')}
            </DataTableCardField>
          )}
          {visibleColumnIds.has('actions') && (
            <DataTableCardField
              label={t('Operations')}
              span={2}
              contentMode='full'
            >
              {renderApiKeyCell(props.row, 'actions')}
            </DataTableCardField>
          )}
        </DataTableCardDetails>
      )}
    </>
  )
}
