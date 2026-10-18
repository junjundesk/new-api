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
import { useAuthStore } from '@/stores/auth-store'

import { AssistantConversation } from './components/assistant-conversation'
import { AssistantInput } from './components/assistant-input'
import {
  AssistantConfigErrorState,
  AssistantDisabledState,
  AssistantHeader,
  AssistantLoadingState,
} from './components/assistant-status-states'
import { ASSISTANT_LIMITS } from './constants'
import { useAssistantChat } from './hooks/use-assistant-chat'
import { useAssistantConfig } from './hooks/use-assistant-config'
import { getAssistantStorageKey } from './lib/history'
import type { AssistantProfile } from './types'

interface AssistantProps {
  /**
   * Which independent assistant profile to render. Defaults to the user
   * profile served at /assistant; every admin page must pass 'admin'.
   */
  profile?: AssistantProfile
}

export function Assistant(props: AssistantProps) {
  const profile = props.profile ?? 'user'
  const userId = useAuthStore((state) => state.auth.user?.id)
  const configQuery = useAssistantConfig(profile)
  const config = configQuery.data

  const chat = useAssistantChat({
    profile,
    storageKey: getAssistantStorageKey(userId, profile),
    maxMessageChars:
      config?.maxMessageChars ?? ASSISTANT_LIMITS.MAX_MESSAGE_CHARS,
    maxHistoryMessages:
      config?.maxHistoryMessages ?? ASSISTANT_LIMITS.MAX_HISTORY_MESSAGES,
    maxHistoryChars:
      config?.maxHistoryChars ?? ASSISTANT_LIMITS.MAX_HISTORY_CHARS,
  })

  function handleRetry(): void {
    configQuery.refetch().catch(() => undefined)
  }

  if (configQuery.isLoading) {
    return <AssistantLoadingState />
  }
  if (configQuery.isError || !config) {
    return <AssistantConfigErrorState onRetry={handleRetry} />
  }
  if (!config.enabled) {
    return <AssistantDisabledState profile={profile} />
  }

  const model = chat.activeModel ?? config.model

  return (
    <div className='relative flex size-full min-h-0 flex-col overflow-hidden'>
      <AssistantHeader
        canClear={chat.messages.length > 0}
        model={model}
        onClear={chat.clearMessages}
        profile={profile}
      />
      <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
        <AssistantConversation
          messages={chat.messages}
          onSelectPrompt={chat.sendMessage}
          profile={profile}
        />
      </div>
      <div className='mx-auto w-full max-w-4xl px-4 pb-4'>
        <AssistantInput
          isStreaming={chat.isStreaming}
          maxMessageChars={config.maxMessageChars}
          onSend={chat.sendMessage}
          onStop={chat.stopStreaming}
        />
      </div>
    </div>
  )
}
