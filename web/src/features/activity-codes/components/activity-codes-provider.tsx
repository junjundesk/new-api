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
import React, { useState } from 'react'

import useDialogState from '@/hooks/use-dialog'

import type { ActivityCode, ActivityCodesDialogType } from '../types'

type ActivityCodesContextType = {
  open: ActivityCodesDialogType | null
  setOpen: (str: ActivityCodesDialogType | null) => void
  currentRow: ActivityCode | null
  setCurrentRow: React.Dispatch<React.SetStateAction<ActivityCode | null>>
  refreshTrigger: number
  triggerRefresh: () => void
}

const ActivityCodesContext = React.createContext<ActivityCodesContextType | null>(
  null
)

export function ActivityCodesProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useDialogState<ActivityCodesDialogType>(null)
  const [currentRow, setCurrentRow] = useState<ActivityCode | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  return (
    <ActivityCodesContext
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        refreshTrigger,
        triggerRefresh,
      }}
    >
      {children}
    </ActivityCodesContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useActivityCodes = () => {
  const activityCodesContext = React.useContext(ActivityCodesContext)

  if (!activityCodesContext) {
    throw new Error(
      'useActivityCodes has to be used within <ActivityCodesProvider>'
    )
  }

  return activityCodesContext
}
