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
import { BarChart, KeyRound, Sparkles, Wallet } from 'lucide-react'
import type { ElementType } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import type { AssistantProfile } from '../types'

interface AssistantEmptyStateProps {
  profile?: AssistantProfile
  onSelectPrompt: (prompt: string) => void
}

interface SuggestionButtonProps {
  icon: ElementType
  label: string
  onSelectPrompt: (prompt: string) => void
}

function SuggestionButton(props: SuggestionButtonProps) {
  const Icon = props.icon
  return (
    <Button
      className='h-auto min-h-11 justify-start gap-2 px-3 py-2.5 text-left whitespace-normal'
      onClick={() => props.onSelectPrompt(props.label)}
      type='button'
      variant='outline'
    >
      <Icon className='text-muted-foreground size-4' aria-hidden='true' />
      <span>{props.label}</span>
    </Button>
  )
}

export function AssistantEmptyState(props: AssistantEmptyStateProps) {
  const { t } = useTranslation()

  return (
    <div className='flex min-h-[min(520px,calc(100svh-20rem))] items-center justify-center px-1 py-8 md:py-12'>
      <div className='grid w-full max-w-2xl gap-5 text-center'>
        <div className='bg-muted/50 text-muted-foreground mx-auto flex size-11 items-center justify-center rounded-xl border'>
          <Sparkles className='size-5' aria-hidden='true' />
        </div>
        <div className='grid gap-2'>
          <h2 className='text-xl font-semibold tracking-tight text-balance md:text-2xl'>
            {t('Ask about your account')}
          </h2>
          <p className='text-muted-foreground mx-auto max-w-lg text-sm leading-6 text-balance'>
            {t(
              'Check your usage, review API keys, or create a key. The assistant only works with your own account data.'
            )}
          </p>
          {props.profile === 'admin' ? (
            <p className='text-muted-foreground mx-auto max-w-lg text-xs leading-5 text-balance'>
              {t(
                "This assistant uses the administrator's separate assistant profile."
              )}
            </p>
          ) : null}
        </div>
        <div className='grid gap-2 sm:grid-cols-2'>
          <SuggestionButton
            icon={Wallet}
            label={t('Show my quota usage over the last 7 days')}
            onSelectPrompt={props.onSelectPrompt}
          />
          <SuggestionButton
            icon={BarChart}
            label={t('Which models did I use most this month?')}
            onSelectPrompt={props.onSelectPrompt}
          />
          <SuggestionButton
            icon={KeyRound}
            label={t('List my API keys')}
            onSelectPrompt={props.onSelectPrompt}
          />
          <SuggestionButton
            icon={Sparkles}
            label={t('Create an API key named test-key for testing')}
            onSelectPrompt={props.onSelectPrompt}
          />
        </div>
      </div>
    </div>
  )
}
