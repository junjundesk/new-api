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
import { USER_LOG_GROUP } from '../constants'
import type { UserLogsQuery } from '../types'

/**
 * 组装用户日志请求：管理员查全站（/api/log），普通用户只查自己
 * （/api/log/self，后端按登录用户隔离）。两者都固定 account 分组，
 * 与「使用日志」的 usage 分组互不串场。
 */
export function buildUserLogsRequest(params: UserLogsQuery, isAdmin: boolean) {
  return {
    path: isAdmin ? '/api/log' : '/api/log/self',
    params: { ...params, log_group: USER_LOG_GROUP },
  }
}
