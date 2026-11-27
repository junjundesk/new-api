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
import type { RowSelectionState } from '@tanstack/react-table'
import { Download, Upload } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
  hasPermission,
} from '@/lib/admin-permissions'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  ChannelConfigDialog,
  type ChannelConfigAction,
} from './dialogs/channel-config-dialog'

export function ChannelConfigActions(props: {
  rowSelection?: RowSelectionState
}) {
  const { t } = useTranslation()
  const currentUser = useAuthStore((state) => state.auth.user)
  const canExport = currentUser?.role === ROLE.SUPER_ADMIN
  const canImport = hasPermission(
    currentUser,
    ADMIN_PERMISSION_RESOURCES.CHANNEL,
    ADMIN_PERMISSION_ACTIONS.SENSITIVE_WRITE
  )
  const [action, setAction] = useState<ChannelConfigAction | null>(null)

  // With server-side pagination the row model only contains the current page.
  // Use stable selection IDs so checked channels on other pages are not lost.
  const selectedIds = Object.entries(props.rowSelection ?? {}).flatMap(
    ([rowId, checked]) => {
      if (!checked || !rowId.startsWith('channel:')) return []
      const id = Number(rowId.slice('channel:'.length))
      return Number.isSafeInteger(id) && id > 0 ? [id] : []
    }
  )

  return (
    <>
      <Button
        variant='outline'
        size='sm'
        title={
          canImport
            ? t('Import JSON')
            : t('No permission to perform this action')
        }
        disabled={!canImport}
        onClick={() => setAction({ mode: 'import' })}
      >
        <Upload aria-hidden='true' />
        <span className='max-sm:sr-only'>{t('Import JSON')}</span>
      </Button>
      {props.rowSelection && (
        <Button
          variant='outline'
          size='sm'
          title={
            canExport
              ? t('Export selected channels')
              : t('Only super admins can export channel keys.')
          }
          disabled={!canExport || !selectedIds.length}
          onClick={() => setAction({ mode: 'export', ids: selectedIds })}
        >
          <Download aria-hidden='true' />
          <span className='max-sm:sr-only'>{t('Export JSON')}</span>
          {selectedIds.length > 0 && <span>({selectedIds.length})</span>}
        </Button>
      )}
      {action && (action.mode === 'export' ? canExport : canImport) && (
        <ChannelConfigDialog action={action} onClose={() => setAction(null)} />
      )}
    </>
  )
}
