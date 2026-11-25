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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useId } from 'react'
import {
  useFieldArray,
  useForm,
  useWatch,
  type UseFormReturn,
} from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'

import { getCostGroups, saveCostGroups, type RevenueItem } from './api'
import { revenueSettingsSchema, type CostSettings } from './schema'

type EditorProps = { items: RevenueItem[]; onClose: () => void }

function GroupFields(props: {
  form: UseFormReturn<CostSettings>
  index: number
  listId: string
  items: RevenueItem[]
  remove: () => void
}) {
  const { t } = useTranslation()
  const form = props.form
  const groupPath = `groups.${props.index}.group` as const
  const group = useWatch({ control: form.control, name: groupPath })
  const models = useFieldArray({
    control: form.control,
    name: `groups.${props.index}.models`,
  })
  const modelList = useId()
  const suggestions = props.items.filter(
    (row) => row.group === group && row.model
  )
  return (
    <fieldset className='space-y-3 rounded-lg border p-3'>
      <legend className='px-1 text-sm font-semibold'>
        {t('Group')} {props.index + 1}
      </legend>
      <div className='grid items-end gap-2 sm:grid-cols-[1fr_1fr_auto]'>
        <label>
          {t('Bound usage group')}
          <Input
            list={props.listId}
            required
            maxLength={255}
            {...form.register(groupPath)}
          />
        </label>
        <label>
          {t('Group cost multiplier')}
          <Input
            required
            inputMode='decimal'
            placeholder='0.6'
            {...form.register(`groups.${props.index}.cost_ratio`)}
          />
        </label>
        <Button type='button' variant='outline' onClick={props.remove}>
          {t('Remove group')}
        </Button>
      </div>
      <datalist id={modelList}>
        {suggestions.map((row) => (
          <option key={row.model} value={row.model} />
        ))}
      </datalist>
      {models.fields.map((m, j) => (
        <div
          key={m.id}
          className='grid items-end gap-2 sm:grid-cols-[1fr_1fr_auto]'
        >
          <label>
            {t('Model ID')}
            <Input
              list={modelList}
              required
              maxLength={255}
              {...form.register(`groups.${props.index}.models.${j}.model`)}
            />
          </label>
          <label>
            {t('Model cost multiplier')}
            <Input
              required
              inputMode='decimal'
              {...form.register(`groups.${props.index}.models.${j}.cost_ratio`)}
            />
          </label>
          <Button
            type='button'
            variant='outline'
            onClick={() => models.remove(j)}
          >
            {t('Remove override')}
          </Button>
        </div>
      ))}
      <Button
        type='button'
        variant='outline'
        disabled={models.fields.length >= 256}
        onClick={() => models.append({ model: '', cost_ratio: '' })}
      >
        {t('Add model override')}
      </Button>
    </fieldset>
  )
}

function CostForm(
  props: EditorProps & { settings: CostSettings; choices: string[] }
) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const listId = useId()
  const form = useForm<CostSettings>({
    resolver: zodResolver(revenueSettingsSchema),
    defaultValues: { groups: props.settings.groups },
  })
  const groups = useFieldArray({ control: form.control, name: 'groups' })
  const mutation = useMutation({
    mutationFn: saveCostGroups,
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: ['revenue-report'] }),
        client.invalidateQueries({ queryKey: ['revenue-cost-groups'] }),
      ])
      toast.success(t('Cost groups saved'))
      props.onClose()
    },
  })
  const choices = [
    ...new Set(
      [...props.choices, ...props.items.map((r) => r.group)].filter(Boolean)
    ),
  ].sort()
  return (
    <form
      className='space-y-3'
      onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
    >
      <fieldset className='space-y-3' disabled={mutation.isPending}>
        <datalist id={listId}>
          {choices.map((g) => (
            <option key={g} value={g} />
          ))}
        </datalist>
        {groups.fields.map((g, i) => (
          <GroupFields
            key={g.id}
            index={i}
            listId={listId}
            form={form}
            items={props.items}
            remove={() => groups.remove(i)}
          />
        ))}
        <Button
          type='button'
          variant='outline'
          disabled={groups.fields.length >= 128}
          onClick={() =>
            groups.append({ group: '', cost_ratio: '1', models: [] })
          }
        >
          {t('Add group binding')}
        </Button>
        {form.formState.errors.groups && (
          <p role='alert' className='text-destructive'>
            {t(
              'Use unique group and model names and multipliers from 0 to 1000 with at most four decimal places.'
            )}
          </p>
        )}
        {mutation.isError && (
          <p role='alert' className='text-destructive'>
            {mutation.error.message}
          </p>
        )}
        <div className='flex flex-wrap justify-end gap-2'>
          <Button type='button' variant='outline' onClick={props.onClose}>
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            disabled={!form.formState.isDirty || mutation.isPending}
          >
            {t('Save cost groups')}
          </Button>
        </div>
      </fieldset>
    </form>
  )
}

export function CostGroupDialog(props: EditorProps) {
  const { t } = useTranslation()
  const settings = useQuery({
    queryKey: ['revenue-cost-groups'],
    queryFn: getCostGroups,
  })
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
    >
      <DialogContent className='max-h-[85vh] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Bind cost groups')}</DialogTitle>
          <DialogDescription>
            {t(
              'Select or enter the exact group recorded in the usage log. A model override replaces its group rate; removing it restores inheritance. Zero cost and loss-making rates are supported.'
            )}
          </DialogDescription>
        </DialogHeader>
        {settings.isPending && <p role='status'>{t('Loading...')}</p>}
        {settings.isError && (
          <Button
            type='button'
            variant='outline'
            onClick={() => void settings.refetch()}
          >
            {t('Retry')}
          </Button>
        )}
        {settings.data && (
          <CostForm
            {...props}
            settings={settings.data}
            choices={settings.data.available_groups}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}
