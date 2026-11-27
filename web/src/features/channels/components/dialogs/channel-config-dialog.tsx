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
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import { Textarea } from '@/components/ui/textarea'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import { exportChannelConfig, importChannelConfig } from '../../api'
import { channelsQueryKeys } from '../../lib/channel-actions'
import { channelConfigSchema } from '../../types'

export type ChannelConfigAction =
  | { mode: 'export'; ids: number[] }
  | { mode: 'import' }

type ChannelConfigDialogProps = {
  action: ChannelConfigAction
  onClose: () => void
}

export function ChannelConfigDialog(props: ChannelConfigDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { copyToClipboard } = useCopyToClipboard()
  // Credentials stay in this dialog, not in a shared query cache or browser storage.
  const [json, setJson] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const isExport = props.action.mode === 'export'

  async function submit() {
    if (busy) return
    setError('')
    setBusy(true)
    try {
      if (props.action.mode === 'export') {
        const config = await exportChannelConfig(props.action.ids)
        setJson(JSON.stringify(config, null, 2))
        return
      }
      let parsed: unknown
      try {
        parsed = JSON.parse(json)
      } catch {
        setError(t('Invalid JSON format'))
        return
      }
      const config = channelConfigSchema.safeParse(parsed)
      if (!config.success) {
        setError(
          t(
            'Use a version 1 channel export with 1 to 100 channels. Each channel needs a name, type, key and models.'
          )
        )
        return
      }
      const count = await importChannelConfig(config.data)
      void queryClient.invalidateQueries({ queryKey: channelsQueryKeys.all })
      toast.success(t('Imported {{count}} channels', { count }))
      props.onClose()
    } catch (cause) {
      setError((cause instanceof Error && cause.message) || t('Request failed'))
    } finally {
      setBusy(false)
    }
  }

  let actionLabel = isExport ? t('Generate JSON') : t('Confirm import')
  if (busy) actionLabel = t('Processing...')

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) props.onClose()
      }}
      showCloseButton={!busy}
      title={isExport ? t('Export selected channels') : t('Import channels')}
      description={
        props.action.mode === 'export'
          ? t(
              'The JSON for {{count}} selected channels includes plaintext credentials. Only copy it to a trusted destination.',
              { count: props.action.ids.length }
            )
          : t(
              'Paste JSON from a channel export. Each entry creates a new channel, even if its name already exists. If any entry is invalid, nothing is imported.'
            )
      }
      footer={
        <>
          <Button variant='outline' disabled={busy} onClick={props.onClose}>
            {isExport && json ? t('Close') : t('Cancel')}
          </Button>
          {isExport && json ? (
            <Button onClick={() => void copyToClipboard(json)}>
              {t('Copy JSON')}
            </Button>
          ) : (
            <Button
              disabled={busy || (!isExport && !json.trim())}
              onClick={submit}
            >
              {actionLabel}
            </Button>
          )}
        </>
      }
    >
      <Field data-invalid={!!error}>
        {(!isExport || json) && (
          <>
            <FieldLabel htmlFor='channel-config-json'>
              {t('Channel JSON')}
            </FieldLabel>
            <Textarea
              id='channel-config-json'
              className='h-56 max-h-[40dvh] font-mono'
              readOnly={isExport}
              disabled={busy}
              spellCheck={false}
              autoComplete='off'
              aria-invalid={!!error}
              aria-describedby={error ? 'channel-config-error' : undefined}
              placeholder='{"version":1,"channels":[{"name":"Example","type":1,"key":"…","models":"gpt-4o","group":"default"}]}'
              value={json}
              onChange={(event) => {
                setJson(event.target.value)
                setError('')
              }}
            />
          </>
        )}
        {error && <FieldError id='channel-config-error'>{error}</FieldError>}
      </Field>
    </Dialog>
  )
}
