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
import type { TFunction } from 'i18next'
import { CalendarClock, Eye, KeyRound } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import dayjs from '@/lib/dayjs'

import type { AssistantCreateApiKeyData } from '../types'
import { ApiKeyRevealDialog } from './api-key-reveal-dialog'

interface CreateApiKeyCardProps {
  data: AssistantCreateApiKeyData
}

function localizeStatus(
  statusText: string | undefined,
  t: TFunction
): string | undefined {
  switch (statusText) {
    case 'enabled':
      return t('Enabled')
    case 'disabled':
      return t('Disabled')
    case 'expired':
      return t('Expired')
    case 'exhausted':
      return t('Exhausted')
    default:
      return statusText
  }
}

export function CreateApiKeyCard(props: CreateApiKeyCardProps) {
  const { t } = useTranslation()
  const [revealOpen, setRevealOpen] = useState(false)
  const data = props.data
  const canReveal = data.id !== undefined

  let expiryText = t('Never expires')
  if (
    !data.neverExpires &&
    data.expiredTime !== undefined &&
    data.expiredTime > 0
  ) {
    expiryText = dayjs(data.expiredTime * 1000).format('YYYY-MM-DD HH:mm')
  }
  const quotaText = data.unlimitedQuota
    ? t('Unlimited quota')
    : t('Quota configured in the API Keys page')
  const statusLabel = localizeStatus(data.statusText, t)

  return (
    <div className='bg-card ring-foreground/10 w-full max-w-md rounded-xl p-3 ring-1'>
      <div className='flex flex-wrap items-center gap-2'>
        <KeyRound className='text-muted-foreground size-4' aria-hidden='true' />
        <span className='text-sm font-medium'>{t('API key created')}</span>
        {statusLabel ? <Badge variant='secondary'>{statusLabel}</Badge> : null}
      </div>
      <dl className='mt-3 grid gap-1.5 text-sm'>
        <div className='flex items-center justify-between gap-3'>
          <dt className='text-muted-foreground'>{t('Name')}</dt>
          <dd className='truncate font-medium'>{data.name}</dd>
        </div>
        <div className='flex items-center justify-between gap-3'>
          <dt className='text-muted-foreground'>{t('API key')}</dt>
          <dd className='truncate font-mono text-xs'>{data.maskedKey}</dd>
        </div>
        <div className='flex items-center justify-between gap-3'>
          <dt className='text-muted-foreground'>{t('Quota')}</dt>
          <dd>{quotaText}</dd>
        </div>
        <div className='flex items-center justify-between gap-3'>
          <dt className='text-muted-foreground'>{t('Expires')}</dt>
          <dd className='inline-flex items-center gap-1'>
            <CalendarClock className='size-3.5' aria-hidden='true' />
            <span>{expiryText}</span>
          </dd>
        </div>
        {data.group ? (
          <div className='flex items-center justify-between gap-3'>
            <dt className='text-muted-foreground'>{t('Group')}</dt>
            <dd className='truncate font-mono text-xs'>{data.group}</dd>
          </div>
        ) : null}
      </dl>
      <div className='mt-3 flex flex-wrap items-center gap-2'>
        <Button
          disabled={!canReveal}
          onClick={() => setRevealOpen(true)}
          size='sm'
          type='button'
          variant='outline'
        >
          <Eye className='size-4' aria-hidden='true' />
          <span>{t('View full key')}</span>
        </Button>
        <span className='text-muted-foreground text-xs'>
          {t('The full key is only shown after an explicit reveal.')}
        </span>
      </div>
      <ApiKeyRevealDialog
        onOpenChange={setRevealOpen}
        open={revealOpen}
        tokenId={data.id}
      />
    </div>
  )
}
