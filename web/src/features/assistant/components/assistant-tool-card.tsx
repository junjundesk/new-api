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
import type { TFunction } from 'i18next'
import {
  CheckCircle2,
  ChevronDown,
  Loader2,
  Wrench,
  XCircle,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { cn } from '@/lib/utils'

import { parseCreateApiKeyResult } from '../lib/tool-data'
import type { AssistantToolPart } from '../types'
import { CreateApiKeyCard } from './create-api-key-card'

interface AssistantToolCardProps {
  part: AssistantToolPart
}

function getToolTitle(name: string, t: TFunction): string {
  switch (name) {
    case 'get_account_summary':
      return t('Account summary')
    case 'get_usage_summary':
      return t('Usage summary')
    case 'list_usage_logs':
      return t('Usage logs')
    case 'list_api_keys':
      return t('API keys')
    case 'create_api_key':
      return t('Create API key')
    case 'set_api_key_status':
      return t('Update API key status')
    default:
      return name.replaceAll('_', ' ')
  }
}

function stringifyToolValue(value: unknown): string {
  if (value === undefined || value === null) return ''
  if (typeof value === 'string') return value
  return JSON.stringify(value, null, 2)
}

function ToolStatusBadge(props: { status: AssistantToolPart['status'] }) {
  const { t } = useTranslation()
  if (props.status === 'running') {
    return (
      <Badge variant='secondary'>
        <Loader2 className='animate-spin' aria-hidden='true' />
        <span>{t('Running')}</span>
      </Badge>
    )
  }
  if (props.status === 'error') {
    return (
      <Badge variant='destructive'>
        <XCircle aria-hidden='true' />
        <span>{t('Failed')}</span>
      </Badge>
    )
  }
  return (
    <Badge variant='secondary'>
      <CheckCircle2 aria-hidden='true' />
      <span>{t('Completed')}</span>
    </Badge>
  )
}

function ToolValueBlock(props: {
  label: string
  value: string
  destructive?: boolean
}) {
  return (
    <div className='space-y-1'>
      <p className='text-muted-foreground text-xs font-medium'>{props.label}</p>
      <pre
        className={cn(
          'bg-background max-h-64 overflow-auto rounded p-2 font-mono text-xs whitespace-pre-wrap',
          props.destructive ? 'text-destructive' : 'text-foreground'
        )}
      >
        {props.value}
      </pre>
    </div>
  )
}

function ToolErrorCard(props: { title: string; error?: string }) {
  const { t } = useTranslation()
  return (
    <div className='border-destructive/30 bg-destructive/5 flex w-full items-start gap-2 rounded-lg border p-2.5 text-sm'>
      <XCircle
        className='text-destructive mt-0.5 size-4 shrink-0'
        aria-hidden='true'
      />
      <div className='min-w-0'>
        <p className='font-medium'>{props.title}</p>
        <p className='text-muted-foreground break-words'>
          {props.error ?? t('The tool call failed')}
        </p>
      </div>
    </div>
  )
}

function GenericToolCard(props: { part: AssistantToolPart }) {
  const { t } = useTranslation()
  const part = props.part
  const title = getToolTitle(part.name, t)
  const argumentsText = stringifyToolValue(part.arguments)
  const resultText = stringifyToolValue(part.result)
  const errorText = part.error ?? ''
  const hasDetails =
    argumentsText !== '' || resultText !== '' || errorText !== ''

  return (
    <Collapsible className='bg-muted/20 w-full rounded-lg border'>
      <CollapsibleTrigger className='group flex w-full items-center gap-2 p-2.5 text-left'>
        <Wrench
          className='text-muted-foreground size-4 shrink-0'
          aria-hidden='true'
        />
        <span className='min-w-0 truncate text-sm font-medium'>{title}</span>
        <ToolStatusBadge status={part.status} />
        <ChevronDown
          className='text-muted-foreground ml-auto size-4 shrink-0 transition-transform group-data-[panel-open]:rotate-180'
          aria-hidden='true'
        />
      </CollapsibleTrigger>
      {hasDetails ? (
        <CollapsibleContent className='space-y-2 border-t p-2.5'>
          {argumentsText !== '' ? (
            <ToolValueBlock label={t('Arguments')} value={argumentsText} />
          ) : null}
          {errorText !== '' ? (
            <ToolValueBlock destructive label={t('Error')} value={errorText} />
          ) : null}
          {resultText !== '' ? (
            <ToolValueBlock label={t('Result')} value={resultText} />
          ) : null}
        </CollapsibleContent>
      ) : null}
    </Collapsible>
  )
}

export function AssistantToolCard(props: AssistantToolCardProps) {
  const { t } = useTranslation()
  const part = props.part

  if (part.name === 'create_api_key') {
    if (part.status === 'error') {
      return <ToolErrorCard error={part.error} title={t('Create API key')} />
    }
    const data = parseCreateApiKeyResult(part.result)
    if (data) return <CreateApiKeyCard data={data} />
  }

  return <GenericToolCard part={part} />
}
