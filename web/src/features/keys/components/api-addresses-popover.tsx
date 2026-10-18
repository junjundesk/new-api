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
import { ChevronDown, Globe, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { useApiInfo } from '@/features/dashboard/hooks/use-status-data'
import { useStatus } from '@/hooks/use-status'

import { normalizeApiBaseUrl } from '../lib/api-address'

type ApiAddressRow = {
  id: string
  route: string
  description: string
  url: string
}

export function ApiAddressesPopover() {
  const { t } = useTranslation()
  const { status, loading } = useStatus()
  const { items } = useApiInfo()

  const origin = typeof window === 'undefined' ? '' : window.location.origin
  const serverAddress =
    typeof status?.server_address === 'string'
      ? status.server_address.trim()
      : ''
  const configured = Array.isArray(items) ? items : []

  const addresses: ApiAddressRow[] = configured.length
    ? configured.map((item, index) => ({
        id: `configured-${index}`,
        route: item.route,
        description: item.description,
        url: normalizeApiBaseUrl(item.url, origin),
      }))
    : [
        {
          id: 'default',
          route: serverAddress ? t('Default API address') : t('Current domain'),
          description: '',
          url: normalizeApiBaseUrl(serverAddress, origin),
        },
      ]

  const visibleAddresses = addresses.filter((address) => address.url)

  return (
    <Popover>
      <PopoverTrigger render={<Button variant='outline' size='sm' />}>
        <Globe aria-hidden='true' />
        {t('API Addresses')}
        <ChevronDown aria-hidden='true' />
      </PopoverTrigger>
      <PopoverContent
        align='end'
        className='max-h-[min(28rem,var(--available-height))] w-96 max-w-[calc(100vw-2rem)] overflow-y-auto'
      >
        <PopoverTitle>{t('API Addresses')}</PopoverTitle>
        {loading ? (
          <div
            role='status'
            className='text-muted-foreground flex items-center gap-2 px-1 py-2 text-xs'
          >
            <Loader2 className='size-3.5 animate-spin' aria-hidden='true' />
            {t('Loading...')}
          </div>
        ) : (
          <ul className='space-y-2'>
            {visibleAddresses.map((address) => (
              <li
                key={address.id}
                className='bg-muted/40 flex items-start gap-2 rounded-lg px-2.5 py-2'
              >
                <div className='min-w-0 flex-1 space-y-0.5'>
                  <div className='text-xs font-medium break-all'>
                    {address.route}
                  </div>
                  <code className='text-muted-foreground block text-xs break-all select-text'>
                    {address.url}
                  </code>
                  {address.description && (
                    <p className='text-muted-foreground text-xs break-words'>
                      {address.description}
                    </p>
                  )}
                </div>
                <CopyButton
                  value={address.url}
                  size='sm'
                  tooltip={t('Copy API URL')}
                  aria-label={`${t('Copy API URL')}: ${address.url}`}
                />
              </li>
            ))}
          </ul>
        )}
      </PopoverContent>
    </Popover>
  )
}
