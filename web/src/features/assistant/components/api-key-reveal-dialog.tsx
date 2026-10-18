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
import { Check, Copy, Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { fetchTokenKey } from '@/features/keys/api'
import { copyToClipboard } from '@/lib/copy-to-clipboard'

import { ASSISTANT_ERROR_KEYS } from '../constants'

interface ApiKeyRevealDialogProps {
  open: boolean
  tokenId?: number
  onOpenChange: (open: boolean) => void
}

/**
 * Fetches the full key only after an explicit reveal request. The key lives in
 * local component state and is discarded when the dialog closes; it is never
 * written to message state or sessionStorage.
 */
export function ApiKeyRevealDialog(props: ApiKeyRevealDialogProps) {
  const { t } = useTranslation()
  const [fullKey, setFullKey] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [errorText, setErrorText] = useState('')
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!props.open) return
    const tokenId = props.tokenId
    if (tokenId === undefined) return
    const requestTokenId: number = tokenId
    let cancelled = false
    setIsLoading(true)
    setErrorText('')

    async function loadFullKey(): Promise<void> {
      try {
        const res = await fetchTokenKey(requestTokenId)
        if (cancelled) return
        if (res.success && res.data?.key) {
          setFullKey(`sk-${res.data.key}`)
          return
        }
        setErrorText(res.message || t(ASSISTANT_ERROR_KEYS.REVEAL_KEY_FAILED))
      } catch {
        if (cancelled) return
        setErrorText(t(ASSISTANT_ERROR_KEYS.REVEAL_KEY_FAILED))
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    }

    loadFullKey().catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [props.open, props.tokenId, t])

  function handleOpenChange(nextOpen: boolean): void {
    if (!nextOpen) {
      setFullKey('')
      setErrorText('')
      setCopied(false)
      setIsLoading(false)
    }
    props.onOpenChange(nextOpen)
  }

  function handleCopy(): void {
    if (fullKey === '') return
    copyToClipboard(fullKey)
      .then((ok) => {
        if (ok) setCopied(true)
      })
      .catch(() => undefined)
  }

  return (
    <Dialog open={props.open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('View full API key')}</DialogTitle>
          <DialogDescription>
            {t(
              'The full key is displayed only here. Store it securely and never share it.'
            )}
          </DialogDescription>
        </DialogHeader>
        {isLoading ? (
          <div className='text-muted-foreground flex items-center gap-2 py-4 text-sm'>
            <Loader2 className='size-4 animate-spin' aria-hidden='true' />
            <span>{t('Loading...')}</span>
          </div>
        ) : null}
        {!isLoading && errorText !== '' ? (
          <p className='text-destructive text-sm' role='alert'>
            {errorText}
          </p>
        ) : null}
        {!isLoading && errorText === '' ? (
          <div className='space-y-2'>
            <input
              aria-label={t('Full API Key')}
              className='bg-muted/50 w-full rounded-md border px-3 py-2 font-mono text-xs outline-none'
              onFocus={(event) => event.currentTarget.select()}
              readOnly
              value={fullKey}
            />
            <Button
              onClick={handleCopy}
              size='sm'
              type='button'
              variant='outline'
            >
              {copied ? (
                <Check className='size-4' aria-hidden='true' />
              ) : (
                <Copy className='size-4' aria-hidden='true' />
              )}
              <span>{copied ? t('Copied!') : t('Copy')}</span>
            </Button>
          </div>
        ) : null}
        <DialogFooter>
          <Button
            onClick={() => handleOpenChange(false)}
            type='button'
            variant='outline'
          >
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
