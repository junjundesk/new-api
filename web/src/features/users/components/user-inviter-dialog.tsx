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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'

import { bindUserInviter } from '../api'
import type { User } from '../types'
import { UserSearchPicker } from './user-search-picker'

interface UserInviterDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /**
   * Users whose inviter is being set. When empty the dialog also asks which
   * user to bind, so it can be opened without a prior selection.
   */
  userIds: number[]
  onSuccess: () => void
}

/**
 * Admin control that points one inviter at every selected user. It only edits
 * the referral relationship; the server never replays signup rewards or past
 * top-up commissions for a manual assignment.
 */
export function UserInviterDialog(props: UserInviterDialogProps) {
  const { t } = useTranslation()
  const [selected, setSelected] = useState<User | null>(null)
  const [target, setTarget] = useState<User | null>(null)
  const [session, setSession] = useState(0)
  const [isBinding, setIsBinding] = useState(false)
  const pickTarget = props.userIds.length === 0

  // Every opening starts from a clean slate so a previous selection cannot be
  // submitted against a different set of users. The session key remounts the
  // pickers, which clears their search text as well.
  useEffect(() => {
    if (props.open) {
      setSelected(null)
      setTarget(null)
      setSession((value) => value + 1)
    }
  }, [props.open])

  let targetIds = props.userIds
  if (pickTarget) {
    targetIds = target ? [target.id] : []
  }

  const handleBind = async () => {
    if (!selected || targetIds.length === 0) return

    setIsBinding(true)
    try {
      const result = await bindUserInviter({
        user_ids: targetIds,
        inviter_id: selected.id,
      })
      if (!result.success) {
        toast.error(result.message || t('Failed to bind inviter'))
        return
      }
      toast.success(
        t('Bound {{count}} user(s) to inviter {{username}}', {
          count: result.data?.updated ?? targetIds.length,
          username: selected.username,
        })
      )
      props.onOpenChange(false)
      props.onSuccess()
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to bind inviter')
      )
    } finally {
      setIsBinding(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Bind Inviter')}
      description={
        pickTarget
          ? t(
              'Pick a user and the inviter to bind them to. An existing inviter binding is overwritten and no invitation reward is granted retroactively.'
            )
          : t(
              'Set the inviter for {{count}} selected user(s). Existing inviter bindings will be overwritten and no invitation reward is granted retroactively.',
              { count: props.userIds.length }
            )
      }
      contentHeight='auto'
      bodyClassName='space-y-3'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={isBinding}
          >
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleBind}
            disabled={!selected || targetIds.length === 0 || isBinding}
          >
            {isBinding ? t('Binding...') : t('Bind Inviter')}
          </Button>
        </>
      }
    >
      {pickTarget ? (
        <div className='grid gap-1.5'>
          <span className='text-sm font-medium'>{t('Invited user')}</span>
          <UserSearchPicker
            key={`target-${session}`}
            label={t('Search invited user')}
            selected={target}
            onSelect={setTarget}
            excludeIds={selected ? [selected.id] : []}
            enabled={props.open}
          />
        </div>
      ) : null}
      <div className='grid gap-1.5'>
        {pickTarget ? (
          <span className='text-sm font-medium'>{t('Inviter')}</span>
        ) : null}
        <UserSearchPicker
          key={`inviter-${session}`}
          label={t('Search users')}
          selected={selected}
          onSelect={setSelected}
          excludeIds={targetIds}
          enabled={props.open}
        />
      </div>
    </Dialog>
  )
}
