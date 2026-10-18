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
import { Square } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  PromptInput,
  PromptInputFooter,
  PromptInputSubmit,
  PromptInputTextarea,
  PromptInputTools,
  type PromptInputMessage,
} from '@/components/ai-elements/prompt-input'
import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import { cn } from '@/lib/utils'

interface AssistantInputProps {
  contextTokens: number
  contextWindow: number
  isStreaming: boolean
  maxMessageChars: number
  onSend: (text: string) => void
  onStop: () => void
}

export function AssistantInput(props: AssistantInputProps) {
  const { t, i18n } = useTranslation()
  // The textarea is controlled: PromptInput resets the form after a submit, and
  // an uncontrolled value would leave React's change tracker stale, so pasting
  // the same prompt again (a common retry after an error) fired no change event
  // and left the send button disabled.
  const [value, setValue] = useState('')
  const canSend = !props.isStreaming && value.trim().length > 0
  const tokenFormat = new Intl.NumberFormat(toIntlLocale(i18n.language), {
    notation: 'compact',
    maximumFractionDigits: 1,
  })
  const contextTokens = Math.max(0, props.contextTokens)
  const contextPercent =
    props.contextWindow > 0
      ? Math.min(100, Math.round((contextTokens / props.contextWindow) * 100))
      : 0
  let contextBarClass = 'bg-primary'
  if (contextPercent >= 90) {
    contextBarClass = 'bg-destructive'
  } else if (contextPercent >= 70) {
    contextBarClass = 'bg-amber-500'
  }

  function handleSubmit(message: PromptInputMessage): void {
    const text = (message.text ?? '').trim()
    if (text === '' || props.isStreaming) return
    setValue('')
    props.onSend(text)
  }

  return (
    <div className='grid shrink-0 gap-2'>
      <PromptInput
        className='relative'
        groupClassName='bg-background/95 dark:bg-background/80 border-border/70 shadow-[0_18px_60px_-32px_rgba(0,0,0,0.65)] ring-1 ring-foreground/5 rounded-xl overflow-hidden transition-all duration-200 focus-within:border-primary/45 focus-within:ring-primary/15'
        onSubmit={handleSubmit}
      >
        <PromptInputTextarea
          autoComplete='off'
          className='min-h-20 px-5 pt-4 pb-3 leading-7 md:min-h-24 md:text-base'
          maxLength={props.maxMessageChars}
          onChange={(event) => setValue(event.currentTarget.value)}
          placeholder={t('Ask about your usage or API keys')}
          value={value}
        />
        <PromptInputFooter className='border-border/60 bg-muted/20 dark:bg-muted/10 border-t px-3 py-2.5'>
          <PromptInputTools>
            {props.contextWindow > 0 ? (
              <div
                className='flex min-w-0 items-center gap-2'
                title={t(
                  'Latest upstream prompt tokens for this conversation. Older messages are trimmed when the context window is full.'
                )}
              >
                <div
                  role='progressbar'
                  aria-label={t('Context usage')}
                  aria-valuemin={0}
                  aria-valuemax={props.contextWindow}
                  aria-valuenow={Math.min(contextTokens, props.contextWindow)}
                  aria-valuetext={t(
                    '{{used}} of {{total}} tokens ({{percent}}%)',
                    {
                      used: tokenFormat.format(contextTokens),
                      total: tokenFormat.format(props.contextWindow),
                      percent: contextPercent,
                    }
                  )}
                  className='bg-muted h-1.5 w-20 overflow-hidden rounded-full'
                >
                  <div
                    className={cn(
                      'h-full rounded-full transition-all',
                      contextBarClass
                    )}
                    style={{ width: `${contextPercent}%` }}
                  />
                </div>
                <span className='text-muted-foreground text-xs tabular-nums'>
                  {t('{{used}} / {{total}} tokens', {
                    used: tokenFormat.format(contextTokens),
                    total: tokenFormat.format(props.contextWindow),
                  })}
                </span>
              </div>
            ) : null}
          </PromptInputTools>
          {props.isStreaming ? (
            <Button
              aria-label={t('Stop')}
              onClick={props.onStop}
              size='icon-sm'
              type='button'
              variant='outline'
            >
              <Square className='size-4' aria-hidden='true' />
            </Button>
          ) : (
            <PromptInputSubmit aria-label={t('Send')} disabled={!canSend} />
          )}
        </PromptInputFooter>
      </PromptInput>
      <p className='text-muted-foreground px-1 text-center text-xs leading-5'>
        {t(
          'Your messages are sent to the AI provider configured by the site administrator. Do not share passwords or full API keys.'
        )}
      </p>
    </div>
  )
}
