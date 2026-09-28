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
import { cn } from '@/lib/utils'

export type MetricTileProps = {
  label: string
  value: string
  valueClassName?: string
}

export function MetricTile(props: MetricTileProps) {
  return (
    <div className='bg-muted/40 flex min-w-0 flex-col gap-0.5 rounded-lg px-2.5 py-2'>
      <span className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase'>
        {props.label}
      </span>
      <span
        className={cn(
          'text-foreground truncate font-mono text-sm font-semibold tabular-nums',
          props.valueClassName
        )}
      >
        {props.value}
      </span>
    </div>
  )
}
