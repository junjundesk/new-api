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
import type { UsageLog } from '@/features/usage-logs/data/schema'

/** 用户日志列表查询参数（后端 /api/log 与 /api/log/self）。 */
export interface UserLogsQuery {
  p: number
  page_size: number
  /** 由 api 层统一写入 log_group=account，调用方无需传。 */
  log_group?: string
  type?: number
  username?: string
  start_timestamp?: number
  end_timestamp?: number
}

export interface UserLogsResponse {
  success: boolean
  message?: string
  data?: {
    items: UsageLog[]
    total: number
  }
}
