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
import { Check, Copy, MonitorSmartphone } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import type { UsageLog } from '../data/schema'
import { parseLogOther } from '../lib/format'

interface LogClientBadgeProps {
  log: UsageLog
  className?: string
}

interface ResolvedClient {
  name: string
  category: string
  variant?: string
  version?: string
  source: string
  confidence: string
  userAgent?: string
}

function resolveClient(props: LogClientBadgeProps): ResolvedClient | null {
  const other = parseLogOther(props.log.other)
  const client = other?.client
  if (!client || !client.name) return null
  return {
    name: client.name,
    category: client.category,
    variant: client.variant,
    version: client.version,
    source: client.source,
    confidence: client.confidence,
    userAgent: other?.user_agent,
  }
}

function ClientDetailRow(props: { term: string; children: React.ReactNode }) {
  return (
    <div className='flex items-start justify-between gap-3'>
      <dt className='text-muted-foreground shrink-0 text-xs'>{props.term}</dt>
      <dd className='min-w-0 text-right text-xs font-medium break-all'>
        {props.children}
      </dd>
    </div>
  )
}

export function LogClientBadge(props: LogClientBadgeProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const [copied, setCopied] = useState(false)

  const client = resolveClient(props)
  if (!client) return null

  const label = client.version
    ? `${client.name} ${client.version}`
    : client.name

  const handleCopyUserAgent = async () => {
    if (!client.userAgent) return
    const success = await copyToClipboard(client.userAgent)
    if (success) {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    }
  }

  const categoryLabel = t(`Client Category ${client.category}`, {
    defaultValue: client.category,
  })

  return (
    <Popover>
      <PopoverTrigger
        render={
          <button
            type='button'
            tabIndex={0}
            aria-haspopup='dialog'
            aria-label={
              t('View client details of {{name}}', { name: client.name })
            }
            className='text-foreground hover:bg-foreground/5 focus-visible:ring-ring inline-flex min-h-7 max-w-full min-w-0 cursor-pointer items-center rounded px-1 text-xs outline-none focus-visible:ring-2'
            onClick={(e) => e.stopPropagation()}
          >
            <span className='inline-flex min-w-0 items-center gap-1.5'>
              <MonitorSmartphone
                className='size-3.5 shrink-0'
                aria-hidden='true'
              />
              <span className='truncate'>{label}</span>
            </span>
          </button>
        }
      />
      <PopoverContent
        side='top'
        align='start'
        className='w-80 gap-3 text-sm'
        onClick={(e) => e.stopPropagation()}
      >
        <div className='flex flex-col gap-0.5'>
          <h2 className='text-sm font-semibold'>{client.name}</h2>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Identified from the original request before upstream rewrite. Client identification is not official authentication.'
            )}
          </p>
        </div>
        <dl className='flex flex-col gap-1.5'>
          <ClientDetailRow term={t('Client Category')}>
            {categoryLabel}
          </ClientDetailRow>
          {client.variant ? (
            <ClientDetailRow term={t('Variant')}>
              {client.variant}
            </ClientDetailRow>
          ) : null}
          {client.version ? (
            <ClientDetailRow term={t('Version')}>
              {client.version}
            </ClientDetailRow>
          ) : null}
          <ClientDetailRow term={t('Identification Source')}>
            {client.source}
          </ClientDetailRow>
          <ClientDetailRow term={t('Identification Status')}>
            {t(`Client Confidence ${client.confidence}`, {
              defaultValue: client.confidence,
            })}
          </ClientDetailRow>
        </dl>
        {client.userAgent ? (
          <div className='border-border/60 flex flex-col gap-1 border-t pt-2'>
            <div className='flex items-center justify-between gap-2'>
              <p className='text-xs font-medium'>{t('Raw User-Agent')}</p>
              <button
                type='button'
                aria-label={t('Copy User-Agent')}
                title={t('Copy User-Agent')}
                className='text-muted-foreground hover:text-foreground inline-flex size-6 items-center justify-center rounded outline-none focus-visible:ring-2'
                onClick={() => {
                  void handleCopyUserAgent()
                }}
              >
                {copied ? (
                  <Check className='size-3.5 text-emerald-500' />
                ) : (
                  <Copy className='size-3.5' />
                )}
              </button>
            </div>
            <p className='text-muted-foreground font-mono text-xs break-all'>
              {client.userAgent}
            </p>
            <p className='text-muted-foreground/70 text-xs'>
              {t(
                'This is the User-Agent received by this gateway. Upstream proxies may have modified it.'
              )}
            </p>
          </div>
        ) : null}
      </PopoverContent>
    </Popover>
  )
}
