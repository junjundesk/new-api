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
import { useTranslation } from 'react-i18next'

import { parseCardStoreUrl } from '@/lib/card-store'
import { formatQuotaWithCurrency } from '@/lib/currency'

import type { WalletCardStore } from '../types'

export function WalletCardStoreSection(props: { store: WalletCardStore }) {
  const { t } = useTranslation()
  const url = parseCardStoreUrl(props.store.url)
  const mixedContent =
    url?.protocol === 'http:' && window.location.protocol === 'https:'
  const money = (quota: number) =>
    formatQuotaWithCurrency(quota, { abbreviate: false, digitsSmall: 6 })
  return (
    <section className='space-y-3' aria-label={t('Card store')}>
      <h3 className='font-medium'>{t('Redemption code tiers')}</h3>
      {props.store.tiers.length === 0 ? (
        <p className='text-muted-foreground text-sm'>
          {t('No redemption code tiers configured yet.')}
        </p>
      ) : (
        <ul className='grid grid-cols-1 gap-2 sm:grid-cols-2'>
          {props.store.tiers.map((tier) => (
            <li key={tier.id} className='min-w-0 rounded-md border p-3'>
              <strong className='break-words'>{tier.name}</strong>
              <div className='mt-1 text-sm'>
                {t('Card face value: {{amount}}', {
                  amount: money(tier.quota),
                })}
              </div>
              <div className='text-muted-foreground text-sm'>
                {t('Bonus credit: {{amount}}', {
                  amount: money(tier.bonus_quota),
                })}
              </div>
            </li>
          ))}
        </ul>
      )}
      {url ? (
        <>
          {!mixedContent && (
            <iframe
              title={t('Card store checkout')}
              src={url.href}
              sandbox='allow-forms allow-scripts'
              referrerPolicy='no-referrer'
              loading='lazy'
              className='h-[min(65vh,36rem)] w-full rounded-md border'
            />
          )}
          <p className='text-muted-foreground text-sm'>
            {mixedContent
              ? t(
                  'An HTTPS wallet cannot embed an HTTP store. Open the store separately.'
                )
              : t(
                  'If the store cannot load or complete checkout here, open it in a new tab.'
                )}{' '}
            <a
              className='text-primary underline'
              href={url.href}
              target='_blank'
              rel='noopener noreferrer'
              referrerPolicy='no-referrer'
            >
              {t('Open card store')}
            </a>
          </p>
        </>
      ) : (
        <p role='alert'>
          {t('Invalid card store address. Contact the administrator.')}
        </p>
      )}
    </section>
  )
}
