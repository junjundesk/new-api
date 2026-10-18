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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { toast } from 'sonner'

import { updateSystemOption } from '../api'
import type { UpdateOptionRequest } from '../types'

export const SETTING_UPDATED_MESSAGE = 'Setting updated successfully'
export const SETTING_UPDATE_FAILED_MESSAGE = 'Failed to update setting'

// Configuration keys that require status refresh
export const STATUS_RELATED_KEYS = new Set([
  'HeaderNavModules',
  'SidebarModulesAdmin',
  'Notice',
  'LogConsumeEnabled',
  'QuotaPerUnit',
  'USDExchangeRate',
  'DisplayInCurrencyEnabled',
  'DisplayTokenStatEnabled',
  'general_setting.quota_display_type',
  'general_setting.custom_currency_symbol',
  'general_setting.custom_currency_exchange_rate',
  'oidc.display_name',
  'assistant_setting.user_enabled',
  'assistant_setting.user_source',
  'assistant_setting.user_base_url',
  'assistant_setting.user_api_key',
  'assistant_setting.user_model',
  'assistant_setting.user_channel_id',
  'assistant_setting.user_channel_model',
  'assistant_setting.user_protocol',
  'assistant_setting.user_context_window',
  'assistant_setting.user_max_output_tokens',
  'assistant_setting.admin_enabled',
  'assistant_setting.admin_source',
  'assistant_setting.admin_base_url',
  'assistant_setting.admin_api_key',
  'assistant_setting.admin_model',
  'assistant_setting.admin_channel_id',
  'assistant_setting.admin_channel_model',
  'assistant_setting.admin_protocol',
  'assistant_setting.admin_context_window',
  'assistant_setting.admin_max_output_tokens',
])

export function useUpdateOption() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (request: UpdateOptionRequest) => updateSystemOption(request),
    onSuccess: (data, variables) => {
      if (data.success) {
        // Always refresh system-options
        queryClient.invalidateQueries({ queryKey: ['system-options'] })

        // If updating frontend-display-related config, also refresh status
        if (STATUS_RELATED_KEYS.has(variables.key)) {
          queryClient.invalidateQueries({ queryKey: ['status'] })
          try {
            window.localStorage.removeItem('status')
          } catch {
            /* empty */
          }
        }

        toast.success(i18next.t(SETTING_UPDATED_MESSAGE))
      } else {
        toast.error(data.message || i18next.t(SETTING_UPDATE_FAILED_MESSAGE))
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || i18next.t(SETTING_UPDATE_FAILED_MESSAGE))
    },
  })
}
