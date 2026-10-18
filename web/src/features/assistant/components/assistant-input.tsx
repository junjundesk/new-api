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

interface AssistantInputProps {
  isStreaming: boolean
  maxMessageChars: number
  onSend: (text: string) => void
  onStop: () => void
}

export function AssistantInput(props: AssistantInputProps) {
  const { t } = useTranslation()
  const [characterCount, setCharacterCount] = useState(0)
  const canSend = !props.isStreaming && characterCount > 0

  function handleSubmit(message: PromptInputMessage): void {
    const text = (message.text ?? '').trim()
    if (text === '' || props.isStreaming) return
    setCharacterCount(0)
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
          onChange={(event) =>
            setCharacterCount(event.currentTarget.value.length)
          }
          placeholder={t('Ask about your usage or API keys')}
        />
        <PromptInputFooter className='border-border/60 bg-muted/20 dark:bg-muted/10 border-t px-3 py-2.5'>
          <PromptInputTools>
            <span className='text-muted-foreground text-xs tabular-nums'>
              {t('{{count}} / {{max}} characters', {
                count: characterCount,
                max: props.maxMessageChars,
              })}
            </span>
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
