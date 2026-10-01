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
// ============================================================================
// Admin Email Type Definitions
// ============================================================================

/** A user selectable as an email recipient. */
export interface EmailRecipient {
  id: number
  username: string
  display_name?: string
  email?: string
}

export interface SendEmailPayload {
  /** Selected user ids; omit or leave empty when all_users is true. */
  user_ids?: number[]
  /** Send to every user (broadcast). */
  all_users?: boolean
  subject: string
  /** HTML body. */
  content: string
}

export interface SendEmailResult {
  /** Number of recipients the message was dispatched to. */
  queued: number
  /** Delivered synchronously (absent on the async broadcast path). */
  sent?: number
  /** Failed synchronously (absent on the async broadcast path). */
  failed?: number
  /** Users skipped because they have no email address. */
  skipped: number
  /** True when the backend sent in the background. */
  async?: boolean
}
