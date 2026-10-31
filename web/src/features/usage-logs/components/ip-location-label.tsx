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
import { useQuery } from '@tanstack/react-query'

import { cn } from '@/lib/utils'

import { getIPLocation } from '../api'

interface IpLocationLabelProps {
  ip: string
  className?: string
}

const IP_LOCATION_STALE_TIME = 24 * 60 * 60 * 1000

/**
 * IP 归属地小字，展示在日志列表 IP 下方。
 * 归属地由后端缓存 24 小时，同一 IP 在前端也只查一次；
 * 查不到（内网地址、第三方接口失败）时不渲染，不给行留空位。
 */
export function IpLocationLabel(props: IpLocationLabelProps) {
  const { data } = useQuery({
    queryKey: ['usage-log-ip-location', props.ip],
    queryFn: () => getIPLocation(props.ip),
    enabled: !!props.ip,
    staleTime: IP_LOCATION_STALE_TIME,
    gcTime: IP_LOCATION_STALE_TIME,
    retry: false,
  })

  if (!data) return null

  return (
    <span
      className={cn(
        'text-muted-foreground block max-w-[180px] truncate text-[10px]! leading-none',
        props.className
      )}
      title={data}
    >
      {data}
    </span>
  )
}
