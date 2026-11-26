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
import { FolderInput, Gift, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'

import { deleteInvalidRedemptions } from '../api'
import { ERROR_MESSAGES } from '../constants'
import { RedemptionBonusRulesDialog } from './redemption-bonus-rules-dialog'
import { RedemptionMigrationDialog } from './redemption-migration-dialog'
import { useRedemptions } from './redemptions-provider'

export function RedemptionsPrimaryButtons() {
  const { t } = useTranslation()
  const { categoryId, setOpen, triggerRefresh } = useRedemptions()
  const [showDeleteInvalidConfirm, setShowDeleteInvalidConfirm] =
    useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [showBonusRules, setShowBonusRules] = useState(false)
  const [showMigration, setShowMigration] = useState(false)

  const handleDeleteInvalid = async () => {
    setIsDeleting(true)
    try {
      const result = await deleteInvalidRedemptions(categoryId)
      if (result.success) {
        const count = result.data || 0
        toast.success(
          t('Successfully deleted {{count}} invalid redemption codes', {
            count,
          })
        )
        triggerRefresh()
        setShowDeleteInvalidConfirm(false)
      } else {
        toast.error(result.message || t(ERROR_MESSAGES.DELETE_INVALID_FAILED))
      }
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <>
      <div className='flex flex-wrap gap-2'>
        {categoryId === 0 && (
          <Button
            size='sm'
            variant='outline'
            className='max-sm:size-8 max-sm:px-0'
            aria-label={t('Migrate codes')}
            title={t('Migrate codes')}
            onClick={() => setShowMigration(true)}
          >
            <FolderInput aria-hidden />
            <span className='hidden sm:inline'>{t('Migrate codes')}</span>
          </Button>
        )}
        <Button
          size='sm'
          variant='outline'
          className='max-sm:size-8 max-sm:px-0'
          aria-label={t('Delete Invalid')}
          title={t('Delete Invalid')}
          onClick={() => setShowDeleteInvalidConfirm(true)}
        >
          <Trash2 className='text-destructive h-4 w-4' />
          <span className='hidden sm:inline'>{t('Delete Invalid')}</span>
        </Button>
        <Button
          size='sm'
          variant='outline'
          className='max-sm:size-8 max-sm:px-0'
          aria-label={t('Bonus rules')}
          title={t('Bonus rules')}
          onClick={() => setShowBonusRules(true)}
        >
          <Gift className='h-4 w-4 text-amber-600 dark:text-amber-400' />
          <span className='hidden sm:inline'>{t('Bonus rules')}</span>
        </Button>
        <Button size='sm' onClick={() => setOpen('create')}>
          <Plus className='h-4 w-4' />
          {t('Generate codes')}
        </Button>
      </div>

      {categoryId === 0 && showMigration && (
        <RedemptionMigrationDialog onClose={() => setShowMigration(false)} />
      )}

      {showBonusRules && (
        <RedemptionBonusRulesDialog onClose={() => setShowBonusRules(false)} />
      )}

      <ConfirmDialog
        destructive
        open={showDeleteInvalidConfirm}
        onOpenChange={setShowDeleteInvalidConfirm}
        handleConfirm={handleDeleteInvalid}
        isLoading={isDeleting}
        className='max-w-md'
        title={t('Delete Invalid Redemption Codes?')}
        desc={
          <>
            {categoryId === undefined
              ? t(
                  'This deletes used, disabled and expired codes in all categories.'
                )
              : t(
                  'This deletes used, disabled and expired codes in the selected category.'
                )}
            <br />
            {t('This action cannot be undone.')}
          </>
        }
        confirmText={t('Delete Invalid')}
      />
    </>
  )
}
