import { useTranslation } from 'react-i18next'

import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'

import { FILTER_ALL } from '../constants'
import { formatBillingRatio, getGroupBillingRatio } from '../lib/model-helpers'
import type { PricingModel } from '../types'

export function ModelBillingRatio(props: {
  model: PricingModel
  selectedGroup?: string
}) {
  const { t } = useTranslation()
  if (
    !props.selectedGroup ||
    props.selectedGroup === FILTER_ALL ||
    !props.model.enable_groups.includes(props.selectedGroup)
  )
    return null
  const ratio = getGroupBillingRatio(props.model, props.selectedGroup)
  const labels = {
    channel_model: t('Model-specific'),
    user_group: t('User group offer'),
    group: t('Group default'),
    mixed: t('By channel'),
  }
  const label = labels[ratio.source]

  return (
    <Popover>
      <PopoverTrigger
        type='button'
        onClick={(event) => event.stopPropagation()}
        className='text-muted-foreground bg-muted/50 hover:bg-muted focus-visible:ring-ring mt-1 inline-flex max-w-full flex-wrap items-center gap-x-1 rounded-md border px-1.5 py-0.5 text-left text-xs focus-visible:ring-2 focus-visible:outline-none'
      >
        <span>
          {t('Billing ratio')}{' '}
          <span className='font-mono tabular-nums'>
            {formatBillingRatio(ratio)}
          </span>
        </span>
        <span>· {label}</span>
      </PopoverTrigger>
      <PopoverContent
        align='start'
        onClick={(event) => event.stopPropagation()}
      >
        <PopoverTitle>
          {t('Billing ratio for {{group}}', { group: props.selectedGroup })}
        </PopoverTitle>
        <PopoverDescription>
          {t(
            'This effective ratio multiplies the base model price. A model-specific ratio replaces the group ratio, including any user group offer; it is not multiplied again.'
          )}
        </PopoverDescription>
        {ratio.min !== ratio.max && (
          <p className='text-sm'>
            {t(
              'Available channels have different ratios. The price range includes fallback channels; the actual price depends on the channel used.'
            )}
          </p>
        )}
        {ratio.min === ratio.max && ratio.source === 'mixed' && (
          <p className='text-sm'>
            {t(
              'Channels use different ratio sources but currently have the same effective price.'
            )}
          </p>
        )}
        {ratio.max === 0 && (
          <p className='text-sm'>
            {t('The effective billing ratio is zero for this group.')}
          </p>
        )}
      </PopoverContent>
    </Popover>
  )
}
