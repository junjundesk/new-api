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
import { useQueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { useCallback, useRef, useState } from 'react'
import { toast } from 'sonner'

import { updateSystemOption } from '../api'
import type { UpdateOptionRequest } from '../types'
import {
  SETTING_UPDATED_MESSAGE,
  SETTING_UPDATE_FAILED_MESSAGE,
  STATUS_RELATED_KEYS,
} from './use-update-option'

type SaveOptionsConfig = {
  successMessage?: string
  emptyMessage?: string
}

/**
 * Saves a set of option changes as one user action.
 *
 * Every changed key still travels in its own request (the option API updates one
 * key at a time), but the caller sees a single outcome: exactly one toast, and a
 * single options refresh after all writes settled, so the form never re-renders
 * from a half-applied server state. Calls made while a save is in flight are
 * ignored, which keeps a double click or a second save entry point from
 * submitting the same changes twice.
 */
export function useSaveOptions() {
  const queryClient = useQueryClient()
  const [isSaving, setIsSaving] = useState(false)
  const savingRef = useRef(false)

  const saveOptions = useCallback(
    async (
      entries: UpdateOptionRequest[],
      config: SaveOptionsConfig = {}
    ): Promise<boolean> => {
      if (savingRef.current) {
        return false
      }

      if (entries.length === 0) {
        toast.info(i18next.t(config.emptyMessage ?? 'No changes to save'))
        return true
      }

      savingRef.current = true
      setIsSaving(true)
      let persisted = true

      try {
        for (const entry of entries) {
          const response = await updateSystemOption(entry)
          if (!response.success) {
            throw new Error(
              response.message || i18next.t(SETTING_UPDATE_FAILED_MESSAGE)
            )
          }
        }
        toast.success(
          i18next.t(config.successMessage ?? SETTING_UPDATED_MESSAGE)
        )
      } catch (error) {
        persisted = false
        toast.error(
          error instanceof Error && error.message
            ? error.message
            : i18next.t(SETTING_UPDATE_FAILED_MESSAGE)
        )
      } finally {
        if (entries.some((entry) => STATUS_RELATED_KEYS.has(entry.key))) {
          queryClient.invalidateQueries({ queryKey: ['status'] })
          try {
            window.localStorage.removeItem('status')
          } catch {
            /* empty */
          }
        }
        await queryClient.invalidateQueries({ queryKey: ['system-options'] })
        savingRef.current = false
        setIsSaving(false)
      }

      return persisted
    },
    [queryClient]
  )

  return { saveOptions, isSaving }
}
