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
import { ArrowRight, Route } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from '@/components/ui/hover-card'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

interface ModelBadgeProps {
  modelName: string
  actualModel?: string
  className?: string
}

interface ModelProvider {
  icon: string
  label: string
}

function resolveModelProvider(modelName: string): ModelProvider | null {
  const model = modelName.toLowerCase()
  const hasAny = (keywords: string[]) =>
    keywords.some((keyword) => model.includes(keyword))

  if (
    hasAny([
      'gpt-',
      'chatgpt-',
      'text-embedding-',
      'omni-moderation',
      'dall-e',
      'whisper',
      'tts-',
    ]) ||
    /\bo[134](?:-|$)/.test(model)
  ) {
    return { icon: 'OpenAI.Color', label: 'OpenAI' }
  }
  if (hasAny(['claude-', 'anthropic'])) {
    return { icon: 'Claude.Color', label: 'Claude' }
  }
  if (hasAny(['gemini-', 'learnlm-'])) {
    return { icon: 'Gemini.Color', label: 'Gemini' }
  }
  if (hasAny(['grok-', 'xai-'])) {
    return { icon: 'Grok.Color', label: 'Grok' }
  }
  if (hasAny(['deepseek-'])) {
    return { icon: 'DeepSeek.Color', label: 'DeepSeek' }
  }
  if (hasAny(['qwen', 'qwq-'])) {
    return { icon: 'Qwen.Color', label: 'Qwen' }
  }
  if (hasAny(['doubao-', 'volcengine'])) {
    return { icon: 'Doubao.Color', label: 'Doubao' }
  }
  if (hasAny(['moonshot-', 'kimi-'])) {
    return { icon: 'Moonshot.Color', label: 'Moonshot' }
  }
  if (hasAny(['minimax', 'abab'])) {
    return { icon: 'Minimax.Color', label: 'MiniMax' }
  }
  if (hasAny(['glm-', 'chatglm', 'cogview', 'cogvideo'])) {
    return { icon: 'Zhipu.Color', label: 'Zhipu' }
  }
  if (hasAny(['mimo-'])) {
    return { icon: 'XiaomiMiMo', label: 'MiMo' }
  }
  if (hasAny(['ernie'])) {
    return { icon: 'Wenxin.Color', label: 'Baidu' }
  }
  if (hasAny(['spark'])) {
    return { icon: 'Spark.Color', label: 'iFlyTek' }
  }
  if (hasAny(['hunyuan'])) {
    return { icon: 'Hunyuan.Color', label: 'Tencent' }
  }
  if (hasAny(['baichuan'])) {
    return { icon: 'Baichuan.Color', label: 'Baichuan' }
  }
  if (hasAny(['internlm'])) {
    return { icon: 'InternLM.Color', label: 'InternLM' }
  }
  if (hasAny(['step-'])) {
    return { icon: 'Stepfun.Color', label: 'StepFun' }
  }
  if (hasAny(['yi-'])) {
    return { icon: 'Yi.Color', label: 'Yi' }
  }
  if (hasAny(['mistral-', 'mixtral-'])) {
    return { icon: 'Mistral.Color', label: 'Mistral' }
  }
  if (hasAny(['llama-', 'meta-'])) {
    return { icon: 'Meta.Color', label: 'Meta' }
  }
  if (hasAny(['command-', 'cohere-'])) {
    return { icon: 'Cohere.Color', label: 'Cohere' }
  }

  return null
}

function ModelBadgeContent(props: ModelBadgeProps) {
  const provider = resolveModelProvider(props.modelName)

  return (
    <StatusBadge
      copyText={props.modelName}
      size='sm'
      showDot={!provider}
      autoColor={provider ? undefined : props.modelName}
      className={cn(
        'border-border/60 bg-muted/30 h-6 max-w-none gap-1.5 rounded-md border px-2 [font-family:var(--font-body)]',
        provider && 'text-foreground',
        props.className
      )}
    >
      <span className='flex max-w-none items-center gap-1.5'>
        {provider && (
          <span
            className='flex h-[18px] w-[18px] shrink-0 items-center justify-center'
            title={provider.label}
            aria-label={provider.label}
          >
            {getLobeIcon(provider.icon, 18)}
          </span>
        )}
        <span className='whitespace-nowrap'>{props.modelName}</span>
      </span>
    </StatusBadge>
  )
}

/**
 * Renders the model a request asked for, plus a conversion flag when the
 * selected channel rewrote it. The upstream model the request was actually sent
 * with stays behind the flag and is revealed on hover, focus or tap, so an
 * enabled model mapping is discoverable without widening the column.
 */
export function ModelBadge(props: ModelBadgeProps) {
  const { t } = useTranslation()

  if (!props.actualModel) {
    return <ModelBadgeContent {...props} />
  }

  const actualProvider = resolveModelProvider(props.actualModel)

  return (
    <HoverCard>
      <HoverCardTrigger
        delay={100}
        closeDelay={80}
        render={
          <button
            type='button'
            aria-haspopup='dialog'
            aria-label={t('Mapped')}
            data-log-model-mapped
            className='focus-visible:ring-ring inline-flex max-w-full min-w-0 cursor-pointer items-center gap-1 rounded-md outline-none focus-visible:ring-2'
            onClick={(e) => e.stopPropagation()}
          />
        }
      >
        <ModelBadgeContent {...props} />
        <Route
          className='text-info size-3.5 shrink-0'
          aria-hidden='true'
          data-log-model-mapped-flag
        />
      </HoverCardTrigger>
      <HoverCardContent
        side='top'
        align='start'
        className='w-80'
        onClick={(e) => e.stopPropagation()}
      >
        <p className='text-muted-foreground text-xs'>
          {t('This channel maps the requested model to the upstream model.')}
        </p>
        <div className='mt-2 flex flex-col gap-2'>
          <div className='flex items-center gap-2'>
            <span className='text-muted-foreground w-20 shrink-0 text-xs'>
              {t('Request Model')}
            </span>
            <span className='flex min-w-0 flex-1 items-center gap-1.5'>
              <span className='truncate font-mono text-xs font-medium'>
                {props.modelName}
              </span>
            </span>
          </div>
          <div className='flex items-center gap-2'>
            <span className='text-muted-foreground w-20 shrink-0 text-xs'>
              {t('Actual Model')}
            </span>
            <span className='flex min-w-0 flex-1 items-center gap-1.5'>
              <ArrowRight
                className='text-muted-foreground size-3 shrink-0'
                aria-hidden='true'
              />
              {actualProvider && (
                <span
                  className='flex h-4 w-4 shrink-0 items-center justify-center'
                  title={actualProvider.label}
                  aria-label={actualProvider.label}
                >
                  {getLobeIcon(actualProvider.icon, 16)}
                </span>
              )}
              <span className='truncate font-mono text-xs font-medium'>
                {props.actualModel}
              </span>
            </span>
          </div>
        </div>
      </HoverCardContent>
    </HoverCard>
  )
}
