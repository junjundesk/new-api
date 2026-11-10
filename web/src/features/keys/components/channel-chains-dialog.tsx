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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { getUserGroups } from '@/lib/api'

import { deleteChannelChain, getChannelChains } from '../api'
import {
  DEFAULT_MAX_ITEMS_BYTES,
  DEFAULT_MAX_MODELS_PER_ITEM,
  DEFAULT_USER_CHANNEL_CHAIN_LIMITS,
  type UserChannelChainLimits,
} from '../lib/user-channel-chain'
import type { ChannelChain } from '../types'
import {
  ChannelChainEditor,
  type ChannelChainEditorHandle,
} from './channel-chain-editor'
import { ChannelChainList } from './channel-chain-list'

type ChannelChainsDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type GroupChainOption = {
  label: string
  value: string
  ratio?: number | string
}

export function ChannelChainsDialog(props: ChannelChainsDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const editorRef = useRef<ChannelChainEditorHandle | null>(null)
  const [editing, setEditing] = useState<'create' | ChannelChain | null>(null)
  const [deleting, setDeleting] = useState<ChannelChain | null>(null)
  const [saving, setSaving] = useState(false)
  const [deletingChain, setDeletingChain] = useState(false)

  const chainsQuery = useQuery({
    queryKey: ['channel-chains'],
    queryFn: async () => {
      const result = await getChannelChains()
      // The management API reports business failures as HTTP 200 with
      // success: false; React Query only surfaces thrown errors as isError.
      if (!result.success) {
        throw new Error(result.message || 'Failed to load group chains')
      }
      return result
    },
    enabled: props.open,
    staleTime: 0,
  })
  const groupsQuery = useQuery({
    queryKey: ['user-groups'],
    queryFn: async () => {
      const result = await getUserGroups()
      if (!result.success) {
        throw new Error(result.message || 'Failed to load user groups')
      }
      return result
    },
    enabled: props.open,
    staleTime: 0,
  })

  const response = chainsQuery.data?.data
  const chains = response?.chains ?? []
  const maxChains =
    response?.max_chains ?? DEFAULT_USER_CHANNEL_CHAIN_LIMITS.maxChains
  const maxGroupsPerChain =
    response?.max_groups_per_chain ??
    DEFAULT_USER_CHANNEL_CHAIN_LIMITS.maxGroupsPerChain
  const tokenUsage = response?.token_usage ?? {}
  const limits: UserChannelChainLimits = {
    maxChains,
    maxGroupsPerChain,
    maxModelsPerItem:
      response?.max_models_per_item ?? DEFAULT_MAX_MODELS_PER_ITEM,
    maxItemsBytes: response?.max_items_bytes ?? DEFAULT_MAX_ITEMS_BYTES,
  }

  const groupOptions = useMemo<GroupChainOption[]>(() => {
    const entries = Object.entries(groupsQuery.data?.data ?? {})
    return entries.flatMap(([group, info]) => {
      if (info.custom_chain) return []
      if (group === 'auto' || group.startsWith('chain:')) return []
      // The backend marks groups that may be used inside a chain; older
      // responses do not carry the flag, so only an explicit false excludes.
      if (info.chain_selectable === false) return []
      let label = group
      if (typeof info.ratio === 'number') {
        label = `${group} (${info.ratio}x)`
      } else if (typeof info.ratio === 'string') {
        label = `${group} (${t('Auto')})`
      }
      return [{ value: group, label, ratio: info.ratio }]
    })
  }, [groupsQuery.data, t])

  const invalidate = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ['channel-chains'] }),
      queryClient.invalidateQueries({ queryKey: ['user-groups'] }),
      queryClient.invalidateQueries({ queryKey: ['keys'] }),
      queryClient.invalidateQueries({
        queryKey: ['token-model-mapping-options'],
      }),
      queryClient.invalidateQueries({ queryKey: ['group-models'] }),
    ])

  const closeEditor = () => {
    setEditing(null)
    setSaving(false)
  }

  const handleDelete = async () => {
    if (!deleting) return
    setDeletingChain(true)
    try {
      const result = await deleteChannelChain(deleting.id)
      if (result.success) {
        const resetCount = result.data?.reset_tokens ?? 0
        if (resetCount > 0) {
          toast.success(
            t(
              'Group chain deleted. {{count}} API key(s) switched to follow your account group.',
              { count: resetCount }
            )
          )
        } else {
          toast.success(t('Group chain deleted'))
        }
        await invalidate()
        setDeleting(null)
      } else {
        toast.error(result.message || t('An unexpected error occurred'))
      }
    } catch {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setDeletingChain(false)
    }
  }

  let description = t(
    'Chain your selectable pricing groups in a custom order and use the chain as an API key group.'
  )
  if (editing) {
    if (editing === 'create') {
      description = t(
        'Group-based chains try each group in order; model-based chains route each requested model to its own ordered groups.'
      )
    } else if (editing.type === 'models') {
      description = t(
        'Pick one group per model row; requests for a model use its row order.'
      )
    } else {
      description = t(
        'Pick pricing groups and order them; requests fall back group by group.'
      )
    }
  }

  let body: React.ReactNode
  if (chainsQuery.isLoading) {
    body = (
      <div
        className='text-muted-foreground py-8 text-center text-sm'
        role='status'
      >
        {t('Loading...')}
      </div>
    )
  } else if (chainsQuery.isError) {
    body = (
      <div className='flex flex-col items-center gap-2 py-8 text-center text-sm'>
        <p className='text-destructive'>{t('Failed to load group chains')}</p>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => chainsQuery.refetch()}
        >
          {t('Retry')}
        </Button>
      </div>
    )
  } else if (editing) {
    body = (
      <ChannelChainEditor
        key={editing === 'create' ? 'create' : editing.id}
        ref={editorRef}
        mode={editing === 'create' ? 'create' : 'edit'}
        chain={editing === 'create' ? undefined : editing}
        open={props.open}
        groupOptions={groupOptions}
        groupOptionsReady={groupsQuery.isSuccess}
        limits={limits}
        onSaved={() => {
          setSaving(false)
          closeEditor()
          void invalidate()
        }}
        onCancel={closeEditor}
        onSubmittingChange={setSaving}
      />
    )
  } else {
    body = (
      <div className='space-y-4'>
        <div className='bg-muted/50 text-muted-foreground space-y-1 rounded-lg p-3 text-xs leading-relaxed'>
          <p>
            {t(
              'A group chain routes each request through the pricing groups in the order you set: when a group is unavailable, the next one is tried automatically.'
            )}
          </p>
          <p>
            {t(
              'Create a chain, then select it as the group of an API key. Renaming or reordering a chain takes effect immediately without touching the key.'
            )}
          </p>
          <p>
            {t(
              'Model-based chains store one group and its model list per row, so a key bound to the chain routes each requested model to that row.'
            )}
          </p>
        </div>
        <ChannelChainList
          chains={chains}
          tokenUsage={tokenUsage}
          onEdit={setEditing}
          onDelete={setDeleting}
        />
        <p className='text-muted-foreground text-xs'>
          {t('Chains: {{used}}/{{max}}', {
            used: chains.length,
            max: maxChains,
          })}
        </p>
      </div>
    )
  }

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={(next) => {
          if (!next) {
            // Never drop the dialog while a save request is still in flight.
            if (saving) return
            closeEditor()
          }
          props.onOpenChange(next)
        }}
        title={editing ? t('Edit group chain') : t('Model Group Chains')}
        description={description}
        contentClassName='sm:max-w-[620px]'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          editing ? (
            <div className='flex w-full justify-end gap-2'>
              <Button
                variant='outline'
                type='button'
                onClick={closeEditor}
                disabled={saving}
              >
                {t('Cancel')}
              </Button>
              <Button
                type='button'
                onClick={() => editorRef.current?.submit()}
                disabled={saving}
              >
                {saving ? t('Saving...') : t('Save chain')}
              </Button>
            </div>
          ) : (
            <div className='flex w-full justify-end'>
              <Button
                type='button'
                size='sm'
                disabled={chains.length >= maxChains}
                onClick={() => setEditing('create')}
              >
                <Plus aria-hidden='true' className='size-4' />
                {t('New chain')}
              </Button>
            </div>
          )
        }
      >
        <div className='space-y-3'>
          {groupsQuery.isError ? (
            <div className='border-destructive/40 bg-destructive/5 flex flex-wrap items-center justify-between gap-2 rounded-lg border p-2 text-xs'>
              <span className='text-destructive'>
                {t('Failed to load user groups')}
              </span>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => groupsQuery.refetch()}
              >
                {t('Retry')}
              </Button>
            </div>
          ) : null}
          {body}
        </div>
      </Dialog>

      <ConfirmDialog
        open={!!deleting}
        onOpenChange={(next) => {
          if (!next) setDeleting(null)
        }}
        title={t('Delete chain')}
        desc={t(
          'Are you sure you want to delete this group chain? API keys bound to it will switch to your account group.'
        )}
        confirmText={t('Delete')}
        destructive
        isLoading={deletingChain}
        handleConfirm={handleDelete}
        className='sm:max-w-md'
      />
    </>
  )
}
