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
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { getGroupModelMembers } from '@/features/models/api'

import { safeJsonParse } from '../utils/json-parser'

type GroupModelDescriptionsDialogProps = {
  groupName: string | null
  value: string
  onOpenChange: (open: boolean) => void
  onApply: (group: string, descriptions: Record<string, string>) => void
}

function parseGroupDescriptions(
  value: string,
  group: string | null
): Record<string, string> {
  if (!group) {
    return {}
  }
  const parsed = safeJsonParse<Record<string, Record<string, string>>>(value, {
    fallback: {},
    silent: true,
  })
  const entries = parsed[group]
  if (!entries || typeof entries !== 'object') {
    return {}
  }
  const descriptions: Record<string, string> = {}
  for (const [modelName, description] of Object.entries(entries)) {
    if (typeof description === 'string') {
      descriptions[modelName] = description
    }
  }
  return descriptions
}

/**
 * Edits the model plaza descriptions of one group. The dialog only writes into
 * the surrounding group form; the page's save button persists the option, so
 * descriptions never open a second save path.
 */
export function GroupModelDescriptionsDialog(
  props: GroupModelDescriptionsDialogProps
) {
  const { t } = useTranslation()
  const groupName = props.groupName
  const open = groupName !== null
  const [drafts, setDrafts] = useState<Record<string, string>>({})
  const [search, setSearch] = useState('')

  const savedDescriptions = useMemo(
    () => parseGroupDescriptions(props.value, groupName),
    [props.value, groupName]
  )

  const { data, isLoading } = useQuery({
    queryKey: ['group-model-members', groupName ?? ''],
    queryFn: () => getGroupModelMembers(groupName ?? ''),
    enabled: open && !!groupName,
  })

  useEffect(() => {
    if (!open) {
      return
    }
    setDrafts(savedDescriptions)
    setSearch('')
  }, [open, savedDescriptions])

  const keyword = search.trim().toLowerCase()
  const members = (data?.data ?? []).filter(
    (member) =>
      keyword === '' || member.model_name.toLowerCase().includes(keyword)
  )

  const handleApply = () => {
    if (!groupName) {
      return
    }
    const descriptions: Record<string, string> = {}
    for (const [modelName, description] of Object.entries(drafts)) {
      if (description.trim() !== '') {
        descriptions[modelName] = description
      }
    }
    props.onApply(groupName, descriptions)
  }

  const renderMembers = () => {
    if (isLoading) {
      return (
        <p className='text-muted-foreground text-sm'>{t('Loading models...')}</p>
      )
    }
    if (members.length === 0) {
      return (
        <p className='text-muted-foreground text-sm'>
          {t('No models in this group yet')}
        </p>
      )
    }
    return (
      <div className='space-y-3'>
        {members.map((member) => (
          <div key={member.model_name} className='space-y-1.5'>
            <Label htmlFor={`group-model-description-${member.model_name}`}>
              {member.model_name}
            </Label>
            <Input
              id={`group-model-description-${member.model_name}`}
              value={drafts[member.model_name] ?? ''}
              placeholder={t('Not configured')}
              onChange={(event) =>
                setDrafts((previous) => ({
                  ...previous,
                  [member.model_name]: event.target.value,
                }))
              }
            />
            {member.description && (
              <p className='text-muted-foreground text-xs'>
                {t('Model Description')}: {member.description}
              </p>
            )}
          </div>
        ))}
      </div>
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={props.onOpenChange}
      title={t('Model descriptions: {{group}}', { group: groupName ?? '' })}
      description={t(
        'Descriptions here are shown in the model plaza when this group is selected; models without one keep their own description.'
      )}
      contentClassName='max-w-2xl'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleApply}>{t('Apply')}</Button>
        </>
      }
    >
      <div className='space-y-2'>
        <p className='text-muted-foreground text-sm'>
          {t('Applies after you save the group settings.')}
        </p>
        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('Search model')}
          aria-label={t('Search model')}
        />
      </div>

      {renderMembers()}
    </Dialog>
  )
}
