/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { RichContent } from '@/components/rich-content'
import { Button } from '@/components/ui/button'

type NoticeDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  notice: string
}

export function NoticeDialog(props: NoticeDialogProps) {
  const { t } = useTranslation()

  if (!props.notice) {
    return null
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('System Notice')}
      description={t('Latest platform updates and notices')}
      contentClassName='sm:max-w-2xl'
      bodyClassName='max-h-[min(60vh,32rem)]'
      footer={
        <Button size='sm' onClick={() => props.onOpenChange(false)}>
          {t('Close')}
        </Button>
      }
    >
      <RichContent breaks content={props.notice} />
    </Dialog>
  )
}
