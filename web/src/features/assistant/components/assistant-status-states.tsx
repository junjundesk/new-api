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
import { Bot, RefreshCw, Sparkles, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Spinner } from '@/components/ui/spinner'

import { ASSISTANT_ERROR_KEYS } from '../constants'
import type { AssistantProfile } from '../types'

export function AssistantLoadingState() {
  const { t } = useTranslation()
  return (
    <div className='text-muted-foreground flex size-full min-h-40 items-center justify-center gap-2 text-sm'>
      <Spinner />
      <span>{t('Loading...')}</span>
    </div>
  )
}

interface AssistantConfigErrorStateProps {
  onRetry: () => void
}

export function AssistantConfigErrorState(
  props: AssistantConfigErrorStateProps
) {
  const { t } = useTranslation()
  return (
    <div className='flex size-full items-center justify-center p-6'>
      <Alert className='max-w-lg' variant='destructive'>
        <AlertTitle>{t(ASSISTANT_ERROR_KEYS.CONFIG_LOAD_FAILED)}</AlertTitle>
        <AlertDescription className='space-y-3'>
          <p>{t('Check your connection and try again.')}</p>
          <Button
            onClick={props.onRetry}
            size='sm'
            type='button'
            variant='outline'
          >
            <RefreshCw className='size-4' aria-hidden='true' />
            <span>{t('Retry')}</span>
          </Button>
        </AlertDescription>
      </Alert>
    </div>
  )
}

interface AssistantDisabledStateProps {
  profile?: AssistantProfile
}

export function AssistantDisabledState(props: AssistantDisabledStateProps) {
  const { t } = useTranslation()
  const isAdmin = (props.profile ?? 'user') === 'admin'
  const description = isAdmin
    ? t(
        'The administrator AI assistant is not configured. Configure it in System Settings > Operations > AI Assistant.'
      )
    : t('The administrator has not enabled the AI assistant on this site.')

  return (
    <div className='flex size-full items-center justify-center p-6'>
      <Empty>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <Bot aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('AI Assistant is unavailable')}</EmptyTitle>
          <EmptyDescription>{description}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    </div>
  )
}

interface AssistantHeaderProps {
  profile?: AssistantProfile
  model?: string
  canClear: boolean
  onClear: () => void
}

export function AssistantHeader(props: AssistantHeaderProps) {
  const { t } = useTranslation()
  const title =
    (props.profile ?? 'user') === 'admin'
      ? t('Admin Assistant')
      : t('AI Assistant')
  return (
    <header className='mx-auto flex w-full max-w-4xl items-center justify-between gap-3 px-4 pt-4 pb-1'>
      <div className='flex min-w-0 items-center gap-2'>
        <Sparkles className='text-primary size-4' aria-hidden='true' />
        <h1 className='truncate text-sm font-semibold'>{title}</h1>
        {props.model ? (
          <Badge className='max-w-40 truncate font-mono' variant='outline'>
            {props.model}
          </Badge>
        ) : null}
      </div>
      <Button
        aria-label={t('Clear chat')}
        disabled={!props.canClear}
        onClick={props.onClear}
        size='sm'
        type='button'
        variant='ghost'
      >
        <Trash2 className='size-4' aria-hidden='true' />
        <span className='hidden sm:inline'>{t('Clear chat')}</span>
      </Button>
    </header>
  )
}
