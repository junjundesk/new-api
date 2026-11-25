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
import { z } from 'zod'

const ratio = z
  .string()
  .regex(/^(?:[0-9]{1,4}(?:[.][0-9]{1,4})?|[.][0-9]{1,4})$/)
  .refine((v) => Number(v) <= 1000)
const name = z
  .string()
  .trim()
  .min(1)
  .refine(
    (v) =>
      new TextEncoder().encode(v).length <= 255 &&
      ![...v].some((c) => c.charCodeAt(0) < 32 || c.charCodeAt(0) === 127)
  )
const group = z.object({
  group: name,
  cost_ratio: ratio,
  models: z
    .array(z.object({ model: name, cost_ratio: ratio }))
    .max(256)
    .refine((rows) => new Set(rows.map((r) => r.model)).size === rows.length),
})
export const revenueSettingsSchema = z.object({
  groups: z
    .array(group)
    .max(128)
    .refine((rows) => new Set(rows.map((r) => r.group)).size === rows.length),
})
export type CostSettings = z.infer<typeof revenueSettingsSchema>
