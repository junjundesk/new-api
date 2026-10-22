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
import { ActivityCodesDeleteDialog } from './activity-codes-delete-dialog'
import { ActivityCodesMutateDrawer } from './activity-codes-mutate-drawer'
import { useActivityCodes } from './activity-codes-provider'
import { ActivityGrantDialog } from './activity-grant-dialog'
import { ActivityGrantsDialog } from './activity-grants-dialog'

export function ActivityCodesDialogs() {
  const { open, setOpen, currentRow } = useActivityCodes()
  const isUpdate = open === 'update'

  return (
    <>
      <ActivityCodesMutateDrawer
        open={open === 'create' || isUpdate}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={isUpdate ? currentRow || undefined : undefined}
      />
      <ActivityCodesDeleteDialog />
      <ActivityGrantsDialog />
      <ActivityGrantDialog />
    </>
  )
}
