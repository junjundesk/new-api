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
import { AlertCircle } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Loader } from '@/components/ai-elements/loader'
import { Message, MessageContent } from '@/components/ai-elements/message'
import { Response } from '@/components/ai-elements/response'

import {
  ASSISTANT_ERROR_KEYS,
  ASSISTANT_MESSAGE_STATUS,
  ASSISTANT_USER_TOOL_NAMES,
} from '../constants'
import { getMessageText } from '../lib/messages'
import type { AssistantChatMessage, AssistantProfile } from '../types'
import { AssistantConfirmationCard } from './assistant-confirmation-card'
import { AssistantToolCard } from './assistant-tool-card'

interface AssistantMessageProps {
  message: AssistantChatMessage
  profile?: AssistantProfile
  onConfirm: (confirmationId: string, approve: boolean) => Promise<void>
}

export function AssistantMessage(props: AssistantMessageProps) {
  const { t } = useTranslation()
  const message = props.message

  if (message.role === 'user') {
    return (
      <Message className='py-2.5' from='user'>
        <MessageContent variant='flat'>
          <p className='break-words whitespace-pre-wrap'>
            {getMessageText(message)}
          </p>
        </MessageContent>
      </Message>
    )
  }

  const isStreaming = message.status === ASSISTANT_MESSAGE_STATUS.STREAMING
  const hasParts = message.parts.length > 0

  return (
    <Message className='py-2.5' from='assistant'>
      <div className='flex w-full min-w-0 flex-1 basis-full flex-col gap-2'>
        {message.parts.map((part) => {
          if (part.type === 'text') {
            return (
              <Response final={!isStreaming} key={part.id}>
                {part.text}
              </Response>
            )
          }
          if (part.type === 'confirmation') {
            return (
              <AssistantConfirmationCard
                key={part.id}
                onConfirm={props.onConfirm}
                part={part}
              />
            )
          }
          if (
            props.profile !== 'admin' &&
            !ASSISTANT_USER_TOOL_NAMES.has(part.name)
          ) {
            return null
          }
          return <AssistantToolCard key={part.id} part={part} />
        })}
        {isStreaming && !hasParts ? (
          <div className='text-muted-foreground flex items-center gap-2 text-sm'>
            <Loader />
            <span>{t('Thinking...')}</span>
          </div>
        ) : null}
        {message.status === ASSISTANT_MESSAGE_STATUS.STOPPED ? (
          <p className='text-muted-foreground text-xs'>
            {t('Generation stopped')}
          </p>
        ) : null}
        {message.status === ASSISTANT_MESSAGE_STATUS.ERROR ? (
          <div
            className='border-destructive/30 bg-destructive/5 text-destructive flex items-start gap-2 rounded-lg border p-2.5 text-sm'
            role='alert'
          >
            <AlertCircle
              className='mt-0.5 size-4 shrink-0'
              aria-hidden='true'
            />
            <span>
              {message.error ?? t(ASSISTANT_ERROR_KEYS.REQUEST_FAILED)}
            </span>
          </div>
        ) : null}
      </div>
    </Message>
  )
}
