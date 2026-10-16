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
/**
 * 用户日志（账户与操作类日志）：登录 / 注册 / 密钥 / 充值 / 管理 / 系统。
 *
 * 与「使用日志」共用 logs 表，靠后端 log_group 过滤区分：
 * account = 充值(1) / 管理(3) / 系统(4) / 登录(7)，usage = 消费(2) / 错误(5) / 退款(6) / 未知(0)。
 */
import { LOG_TYPE_ENUM } from '@/features/usage-logs/constants'

/** 后端 logs 表的账户类日志分组标识。 */
export const USER_LOG_GROUP = 'account'

/** 本页展示的日志类型。 */
export const USER_LOG_TYPE_VALUES = [
  LOG_TYPE_ENUM.TOPUP,
  LOG_TYPE_ENUM.MANAGE,
  LOG_TYPE_ENUM.SYSTEM,
  LOG_TYPE_ENUM.LOGIN,
] as const

/**
 * 类型筛选选项。'0' 是后端「全部类型」哨兵值，因此不出现「Unknown」项。
 */
export const USER_LOG_TYPE_FILTERS = [
  { label: 'All Types', value: '0' },
  { label: 'Top-up', value: String(LOG_TYPE_ENUM.TOPUP) },
  { label: 'Manage', value: String(LOG_TYPE_ENUM.MANAGE) },
  { label: 'System', value: String(LOG_TYPE_ENUM.SYSTEM) },
  { label: 'Login', value: String(LOG_TYPE_ENUM.LOGIN) },
] as const

/** 未指定时间范围时默认查询最近天数。 */
export const USER_LOG_DEFAULT_RANGE_DAYS = 7
