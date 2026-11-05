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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button, buttonVariants } from '@/components/ui/button'

type ModelSpecificRatioNoticeDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Name shown to the user: the group key, or a custom chain's display name. */
  groupLabel: string
  /** Group filter for the model square; undefined when the group has no square page. */
  pricingGroup?: string
  /** False when the model square is disabled, so the copy must not send users there. */
  showModelSquareLink?: boolean
}

export function ModelSpecificRatioNoticeDialog(
  props: ModelSpecificRatioNoticeDialogProps
) {
  const { t } = useTranslation()

  return (
    <AlertDialog open={props.open} onOpenChange={props.onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader className='text-start'>
          <AlertDialogTitle>
            {t('This group has model-specific ratios')}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'Models with a model-specific ratio use their own ratio instead of the group ratio.'
            )}{' '}
            {props.showModelSquareLink &&
              t('The model square shows the exact ratio.')}{' '}
            <span className='text-foreground font-semibold'>
              {props.groupLabel}
            </span>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          {props.showModelSquareLink && (
            <Link
              to='/pricing'
              search={{ group: props.pricingGroup }}
              target='_blank'
              rel='noopener noreferrer'
              onClick={() => props.onOpenChange(false)}
              className={buttonVariants({ variant: 'outline' })}
            >
              {t('View in Model Square')}
            </Link>
          )}
          <Button onClick={() => props.onOpenChange(false)}>{t('OK')}</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
