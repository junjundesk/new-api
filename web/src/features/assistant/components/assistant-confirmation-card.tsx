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
import { Check, ChevronDown, Loader2, ShieldAlert, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'

import {
  ASSISTANT_CONFIRMATION_STATUS,
  ASSISTANT_ERROR_KEYS,
} from '../constants'
import type {
  AssistantConfirmationPart,
  AssistantConfirmationStatus,
} from '../types'

interface AssistantConfirmationCardProps {
  part: AssistantConfirmationPart
  onConfirm: (confirmationId: string, approve: boolean) => Promise<void>
}

function formatRemainingTime(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${minutes}:${String(seconds).padStart(2, '0')}`
}

function stringifyParams(value: unknown): string {
  if (value === undefined || value === null) return ''
  if (typeof value === 'string') return value
  return JSON.stringify(value, null, 2)
}

function RiskBadge(props: { risk: string }) {
  const { t } = useTranslation()
  if (props.risk === 'destructive') {
    return <Badge variant='destructive'>{t('Destructive operation')}</Badge>
  }
  if (props.risk === 'write') {
    return <Badge variant='secondary'>{t('Write operation')}</Badge>
  }
  if (props.risk === '') return null
  return <Badge variant='outline'>{props.risk}</Badge>
}

function ConfirmationStatusBadge(props: {
  status: AssistantConfirmationStatus
}) {
  const { t } = useTranslation()
  if (props.status === 'approved') {
    return <Badge variant='secondary'>{t('Approved')}</Badge>
  }
  if (props.status === 'denied') {
    return <Badge variant='destructive'>{t('Denied')}</Badge>
  }
  if (props.status === 'expired') {
    return <Badge variant='outline'>{t('Expired')}</Badge>
  }
  return null
}

export function AssistantConfirmationCard(
  props: AssistantConfirmationCardProps
) {
  const { t } = useTranslation()
  const part = props.part
  const [nowSeconds, setNowSeconds] = useState(() =>
    Math.floor(Date.now() / 1000)
  )
  const [submitting, setSubmitting] = useState<'approve' | 'deny' | null>(null)
  const [errorText, setErrorText] = useState('')

  useEffect(() => {
    if (part.status !== ASSISTANT_CONFIRMATION_STATUS.PENDING) return
    if (part.expiresAt === undefined) return
    const timer = window.setInterval(() => {
      setNowSeconds(Math.floor(Date.now() / 1000))
    }, 1000)
    return () => window.clearInterval(timer)
  }, [part.expiresAt, part.status])

  const remainingSeconds =
    part.expiresAt === undefined
      ? undefined
      : Math.max(0, part.expiresAt - nowSeconds)
  const expiredLocally =
    part.status === ASSISTANT_CONFIRMATION_STATUS.PENDING &&
    remainingSeconds !== undefined &&
    remainingSeconds <= 0
  const isPending =
    part.status === ASSISTANT_CONFIRMATION_STATUS.PENDING && !expiredLocally
  const paramsText = stringifyParams(part.params)

  function handleDecision(approve: boolean): void {
    if (!isPending || submitting !== null) return
    setErrorText('')
    setSubmitting(approve ? 'approve' : 'deny')
    props.onConfirm(part.confirmationId, approve).catch((error: unknown) => {
      setSubmitting(null)
      const message = error instanceof Error ? error.message.trim() : ''
      setErrorText(
        message === '' ? t(ASSISTANT_ERROR_KEYS.CONFIRM_FAILED) : message
      )
    })
  }

  let resolvedStatus: AssistantConfirmationStatus | null = null
  if (!isPending) {
    resolvedStatus = expiredLocally ? 'expired' : part.status
  }

  return (
    <div className='bg-card ring-foreground/10 w-full max-w-xl rounded-xl p-3 ring-1'>
      <div className='flex flex-wrap items-center gap-2'>
        <ShieldAlert
          className='size-4 shrink-0 text-amber-600'
          aria-hidden='true'
        />
        <span className='text-sm font-medium'>
          {t('Confirmation required')}
        </span>
        <RiskBadge risk={part.risk} />
        {resolvedStatus ? (
          <ConfirmationStatusBadge status={resolvedStatus} />
        ) : null}
      </div>
      {part.summary !== '' ? (
        <p className='text-muted-foreground mt-2 text-sm break-words'>
          {part.summary}
        </p>
      ) : null}
      <dl className='mt-3 grid gap-1.5 text-sm'>
        <div className='flex items-center justify-between gap-3'>
          <dt className='text-muted-foreground'>{t('Operation')}</dt>
          <dd className='truncate font-mono text-xs'>{part.operationId}</dd>
        </div>
      </dl>
      {paramsText !== '' ? (
        <Collapsible className='mt-2 rounded-lg border'>
          <CollapsibleTrigger className='group flex w-full items-center gap-2 p-2.5 text-left'>
            <ChevronDown
              className='text-muted-foreground size-4 shrink-0 transition-transform group-data-[panel-open]:rotate-180'
              aria-hidden='true'
            />
            <span className='text-sm font-medium'>{t('Parameters')}</span>
          </CollapsibleTrigger>
          <CollapsibleContent className='border-t p-2.5'>
            <pre className='bg-background max-h-64 overflow-auto rounded p-2 font-mono text-xs whitespace-pre-wrap'>
              {paramsText}
            </pre>
          </CollapsibleContent>
        </Collapsible>
      ) : null}
      {isPending ? (
        <div className='mt-3 flex flex-wrap items-center gap-2'>
          <Button
            disabled={submitting !== null}
            onClick={() => handleDecision(true)}
            size='sm'
            type='button'
          >
            {submitting === 'approve' ? (
              <Loader2 className='size-4 animate-spin' aria-hidden='true' />
            ) : (
              <Check className='size-4' aria-hidden='true' />
            )}
            <span>{t('Approve')}</span>
          </Button>
          <Button
            disabled={submitting !== null}
            onClick={() => handleDecision(false)}
            size='sm'
            type='button'
            variant='outline'
          >
            {submitting === 'deny' ? (
              <Loader2 className='size-4 animate-spin' aria-hidden='true' />
            ) : (
              <X className='size-4' aria-hidden='true' />
            )}
            <span>{t('Deny')}</span>
          </Button>
          {remainingSeconds === undefined ? null : (
            <span className='text-muted-foreground text-xs tabular-nums'>
              {t('Expires in')} {formatRemainingTime(remainingSeconds)}
            </span>
          )}
        </div>
      ) : null}
      {errorText !== '' ? (
        <p className='text-destructive mt-2 text-sm' role='alert'>
          {errorText}
        </p>
      ) : null}
    </div>
  )
}
