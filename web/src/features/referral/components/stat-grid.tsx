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
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

export interface StatItem {
  label: string
  value: string
  hint?: string
}

interface StatGridProps {
  items: StatItem[]
  loading?: boolean
}

/**
 * Row of headline numbers at the top of the referral pages.
 */
export function StatGrid(props: StatGridProps) {
  return (
    <div className='grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-[repeat(auto-fit,minmax(150px,1fr))]'>
      {props.items.map((item) => (
        <Card key={item.label} data-card-hover='false' className='py-0'>
          <CardContent className='p-3 sm:p-4'>
            <div className='text-muted-foreground truncate text-xs'>
              {item.label}
            </div>
            {props.loading ? (
              <Skeleton className='mt-1.5 h-7 w-16' />
            ) : (
              <div className='mt-0.5 truncate text-xl font-semibold tabular-nums sm:text-2xl'>
                {item.value}
              </div>
            )}
            {item.hint ? (
              <div className='text-muted-foreground truncate text-xs'>
                {item.hint}
              </div>
            ) : null}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
