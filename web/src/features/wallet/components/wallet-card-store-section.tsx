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
import { GiftIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { parseCardStoreUrl } from '@/lib/card-store'
import { formatQuotaWithCurrency } from '@/lib/currency'

import type { WalletCardStore } from '../types'

const tierAmountFormat = { abbreviate: false, digitsSmall: 6 }

function CardStoreTiers(props: Pick<WalletCardStore, 'tiers'>) {
  const { t } = useTranslation()
  const title = t('Redemption code tiers')
  return (
    <div className='flex flex-col gap-2'>
      <h3 className='font-medium'>{title}</h3>
      <ul
        aria-label={title}
        className='grid grid-cols-2 gap-2 @xs:grid-cols-3 @md:grid-cols-4 @2xl:grid-cols-6 @4xl:grid-cols-8'
      >
        {props.tiers.map((tier) => {
          const faceValue = formatQuotaWithCurrency(
            tier.quota,
            tierAmountFormat
          )
          const name = tier.name.trim()
          let showName = name !== '' && name !== faceValue
          if (showName && /^(?:\d+|\d{1,3}(?:,\d{3})+)(?:\.\d+)?$/.test(name)) {
            // Compare against the displayed denomination, not the raw quota units.
            const denomination = formatQuotaWithCurrency(tier.quota, {
              ...tierAmountFormat,
              showSymbol: false,
              locale: 'en-US',
            })
            showName =
              Number(name.replaceAll(',', '')) !==
              Number(denomination.replaceAll(',', ''))
          }
          return (
            <li
              key={tier.id}
              className='bg-muted/20 flex min-h-12 min-w-0 flex-col justify-center gap-0.5 rounded-md border px-2.5 py-2 wrap-anywhere tabular-nums'
            >
              {showName && (
                <span className='text-muted-foreground text-xs'>{name}</span>
              )}
              <strong className='text-base leading-tight font-semibold'>
                <span className='sr-only'>{t('Face value')} </span>
                {faceValue}
              </strong>
              {tier.bonus_quota > 0 && (
                <span className='text-muted-foreground flex items-center gap-1 text-xs font-medium'>
                  <HugeiconsIcon
                    icon={GiftIcon}
                    aria-hidden='true'
                    className='text-warning size-3 shrink-0'
                  />
                  <span>
                    {t('{{amount}} bonus', {
                      amount: formatQuotaWithCurrency(
                        tier.bonus_quota,
                        tierAmountFormat
                      ),
                    })}
                  </span>
                </span>
              )}
            </li>
          )
        })}
      </ul>
    </div>
  )
}

export function WalletCardStoreSection(props: { store: WalletCardStore }) {
  const { t } = useTranslation()
  const url = parseCardStoreUrl(props.store.url)
  const mixedContent =
    url?.protocol === 'http:' && window.location.protocol === 'https:'
  // A scripted frame with its real origin must never be mounted on the wallet's own origin.
  const sameOrigin = url?.origin === window.location.origin
  let checkoutHint = t(
    'If the store cannot load or complete checkout here, open it in a new tab.'
  )
  if (mixedContent) {
    checkoutHint = t(
      'An HTTPS wallet cannot embed an HTTP store. Open the store separately.'
    )
  } else if (sameOrigin) {
    checkoutHint = t(
      'This store shares the wallet origin. Open it in a new tab to keep checkout separate.'
    )
  }
  return (
    <section
      className='@container flex flex-col gap-3'
      aria-label={t('Card store')}
    >
      {props.store.show_tiers === true && props.store.tiers.length > 0 && (
        <CardStoreTiers tiers={props.store.tiers} />
      )}
      {url ? (
        <>
          {/* eslint-disable react/iframe-missing-sandbox -- The store needs its own origin for checkout storage; wallet-origin URLs take the standalone link below. */}
          {!mixedContent && !sameOrigin && (
            <iframe
              title={t('Card store checkout')}
              src={url.href}
              sandbox='allow-forms allow-scripts allow-same-origin'
              referrerPolicy='no-referrer'
              loading='lazy'
              className='h-[min(65vh,36rem)] w-full rounded-md border'
            />
          )}
          {/* eslint-enable react/iframe-missing-sandbox */}
          <p className='text-muted-foreground text-sm'>
            {checkoutHint}{' '}
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
