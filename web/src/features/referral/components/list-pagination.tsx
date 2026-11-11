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
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { pageCount } from '../lib/format'

interface ListPaginationProps {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
}

/**
 * Previous/next pager for the short referral lists. It hides itself when
 * everything fits on one page.
 */
export function ListPagination(props: ListPaginationProps) {
  const { t } = useTranslation()
  const pages = pageCount(props.total, props.pageSize)
  if (pages <= 1) {
    return null
  }

  return (
    <div className='flex items-center justify-end gap-2 pt-3'>
      <span className='text-muted-foreground text-xs tabular-nums'>
        {t('Page {{current}} of {{total}}', {
          current: props.page,
          total: pages,
        })}
      </span>
      <Button
        variant='outline'
        size='icon-sm'
        aria-label={t('Previous page')}
        disabled={props.page <= 1}
        onClick={() => props.onPageChange(props.page - 1)}
      >
        <ChevronLeft aria-hidden='true' />
      </Button>
      <Button
        variant='outline'
        size='icon-sm'
        aria-label={t('Next page')}
        disabled={props.page >= pages}
        onClick={() => props.onPageChange(props.page + 1)}
      >
        <ChevronRight aria-hidden='true' />
      </Button>
    </div>
  )
}
