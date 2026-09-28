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
import { Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

const CACHE_DOC_LINKS = [
  {
    label: 'OpenAI',
    href: 'https://developers.openai.com/api/docs/guides/prompt-caching',
  },
  {
    label: 'Anthropic',
    href: 'https://platform.claude.com/docs/en/build-with-claude/prompt-caching',
  },
]

export function CacheMetricsPopover() {
  const { t } = useTranslation()
  return (
    <Popover>
      <PopoverTrigger render={<Button variant='outline' size='sm' />}>
        <Info className='size-3.5' aria-hidden />
        {t('Cache metrics explained')}
      </PopoverTrigger>
      <PopoverContent align='end' className='w-96 gap-3 text-sm'>
        <p className='font-medium'>{t('Cache metrics explained')}</p>
        <div className='text-muted-foreground flex flex-col gap-2 text-xs'>
          <p>
            {t(
              'Cache hit is cached input tokens divided by total input tokens, summed across successful requests in this group. Output tokens are excluded.'
            )}
          </p>
          <p>
            {t(
              'Coding cache uses the same calculation, only for Codex, Claude Code, Pi, OpenCode, OMP, ZCode, DeepSeek Harness (DSH), and Open Design clients.'
            )}
          </p>
          <p>
            {t(
              'Observed multiplier = input cost \u00f7 the 0% cache reference cost. Reference-adjusted multiplier uses your selected cache scenario instead. Both keep cache-write costs fixed and require 90% input cost coverage.'
            )}
          </p>
          <p>
            {t(
              'Only valid upstream usage is included. No data is shown as \u2014; a measured zero is 0%. Statistics cover all users in the group.'
            )}
          </p>
        </div>
        <div className='flex items-center gap-3 text-xs'>
          {CACHE_DOC_LINKS.map((link) => (
            <a
              key={link.label}
              href={link.href}
              target='_blank'
              rel='noreferrer'
              className='text-primary underline underline-offset-4'
            >
              {link.label}
            </a>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}
