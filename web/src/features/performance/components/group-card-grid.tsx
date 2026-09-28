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
import type { PerfGroupPricing } from '@/features/performance-metrics/lib/pricing'
import type { PerformanceGroupSummary } from '@/features/performance-metrics/types'

import type { AvailabilityWindow } from '../lib/series'
import { groupDisplayName, type GroupSortKey } from '../lib/sort'
import { GroupCard } from './group-card'
import { GroupMoveControls } from './group-move-controls'

export type GroupCardGridProps = {
  groups: PerformanceGroupSummary[]
  pricingByGroup: Map<string, PerfGroupPricing | null>
  window: AvailabilityWindow
  sortKey: GroupSortKey
  customOrder: string[]
  allGroups: string[]
  onMove: (group: string, delta: number) => void
  onOpenModels: (group: string) => void
}

export function GroupCardGrid(props: GroupCardGridProps) {
  const visibleOrder = props.groups.map((group) => group.group)
  const custom = props.sortKey === 'custom'

  return (
    <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
      {props.groups.map((group, index) => {
        const visibleIndex = visibleOrder.indexOf(group.group)
        return (
          <div key={group.group} className='flex min-w-0 flex-col gap-2'>
            {custom ? (
              <GroupMoveControls
                groupName={groupDisplayName(group)}
                disabledUp={visibleIndex === 0}
                disabledDown={visibleIndex === visibleOrder.length - 1}
                onMove={(delta) => props.onMove(group.group, delta)}
              />
            ) : null}
            <GroupCard
              summary={group}
              pricing={props.pricingByGroup.get(group.group) ?? null}
              window={props.window}
              entranceDelayMs={Math.min(60 * index, 240)}
              onOpenModels={() => props.onOpenModels(group.group)}
            />
          </div>
        )
      })}
    </div>
  )
}
