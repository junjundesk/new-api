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
import type { Table } from '@tanstack/react-table'
import { UserPlus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import type { User } from '../types'
import { UserInviterDialog } from './user-inviter-dialog'
import { useUsers } from './users-provider'

interface DataTableBulkActionsProps {
  table: Table<User>
}

export function DataTableBulkActions({ table }: DataTableBulkActionsProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useUsers()
  const [inviterDialogOpen, setInviterDialogOpen] = useState(false)
  const selectedIds = table
    .getSelectedRowModel()
    .rows.map((row) => row.original.id)

  return (
    <>
      <BulkActionsToolbar table={table} entityName='user'>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={() => setInviterDialogOpen(true)}
                className='size-8'
                aria-label={t('Bind inviter for selected users')}
                title={t('Bind inviter for selected users')}
              />
            }
          >
            <UserPlus />
            <span className='sr-only'>
              {t('Bind inviter for selected users')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Bind inviter for selected users')}</p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      <UserInviterDialog
        open={inviterDialogOpen}
        onOpenChange={setInviterDialogOpen}
        userIds={selectedIds}
        onSuccess={() => {
          table.resetRowSelection()
          triggerRefresh()
        }}
      />
    </>
  )
}
