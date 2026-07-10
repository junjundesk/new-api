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
import * as React from 'react'

import {
  Tabs,
  TabsContent,
  TabsList as ShadcnTabsList,
  TabsTrigger as ShadcnTabsTrigger,
} from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

function TabsList({
  className,
  ...props
}: React.ComponentProps<typeof ShadcnTabsList>) {
  return (
    <ShadcnTabsList
      data-control-size='default'
      className={cn(
        'group-data-horizontal/tabs:h-7 sm:group-data-horizontal/tabs:h-8',
        className
      )}
      {...props}
    />
  )
}

function TabsTrigger({
  className,
  ...props
}: React.ComponentProps<typeof ShadcnTabsTrigger>) {
  return (
    <ShadcnTabsTrigger
      className={cn('text-[0.8rem] sm:text-sm', className)}
      {...props}
    />
  )
}

export { Tabs, TabsContent, TabsList, TabsTrigger }
