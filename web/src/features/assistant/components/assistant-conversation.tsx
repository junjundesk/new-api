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
import {
  Conversation,
  ConversationContent,
  ConversationScrollButton,
} from '@/components/ai-elements/conversation'

import type { AssistantChatMessage, AssistantProfile } from '../types'
import { AssistantEmptyState } from './assistant-empty-state'
import { AssistantMessage } from './assistant-message'

interface AssistantConversationProps {
  profile?: AssistantProfile
  messages: AssistantChatMessage[]
  onSelectPrompt: (prompt: string) => void
}

export function AssistantConversation(props: AssistantConversationProps) {
  let content
  if (props.messages.length === 0) {
    content = (
      <AssistantEmptyState
        onSelectPrompt={props.onSelectPrompt}
        profile={props.profile}
      />
    )
  } else {
    content = props.messages.map((message) => (
      <AssistantMessage key={message.id} message={message} />
    ))
  }

  return (
    <Conversation>
      <ConversationContent className='p-0'>
        <div className='mx-auto w-full max-w-4xl px-4 py-4'>{content}</div>
      </ConversationContent>
      <ConversationScrollButton />
    </Conversation>
  )
}
