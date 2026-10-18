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
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it } from 'vitest'

import { AssistantToolCard } from '../components/assistant-tool-card'
import type { AssistantToolPart } from '../types'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

function renderTool(part: AssistantToolPart) {
  render(
    <I18nextProvider i18n={i18n}>
      <AssistantToolCard part={part} />
    </I18nextProvider>
  )
}

function createTool(
  name: string,
  argumentsValue: unknown,
  result: unknown
): AssistantToolPart {
  return {
    id: name,
    type: 'tool',
    toolCallId: name,
    name,
    arguments: argumentsValue,
    status: 'done',
    result,
  }
}

afterEach(() => {
  cleanup()
})

describe('generic operation tool cards', () => {
  it('shows operation, HTTP status and data for execute_operation', () => {
    renderTool(
      createTool(
        'execute_operation',
        { operation_id: 'POST /api/token' },
        {
          status_code: 200,
          success: true,
          data: { id: 1, name: 'test-key' },
        }
      )
    )

    expect(screen.getByText('Execute operation')).toBeVisible()
    expect(screen.getByText('POST /api/token')).toBeVisible()
    expect(screen.getByText('HTTP 200 · Success')).toBeVisible()
    expect(screen.getAllByText(/test-key/).length).toBeGreaterThan(0)
  })

  it('shows the succeeded count for batch_operation', () => {
    renderTool(
      createTool(
        'batch_operation',
        { operation_id: 'POST /api/token' },
        { total: 3, succeeded: 2, failed: 1, results: [{ user_id: 1 }] }
      )
    )

    expect(screen.getByText('Batch operation')).toBeVisible()
    expect(screen.getByText('Succeeded')).toBeVisible()
    expect(screen.getByText('2 / 3')).toBeVisible()
  })

  it('shows the table and row count for admin_db_query', () => {
    renderTool(
      createTool(
        'admin_db_query',
        { table: 'users' },
        { table: 'users', count: 2, rows: [{ id: 1 }, { id: 2 }] }
      )
    )

    expect(screen.getByText('Database query')).toBeVisible()
    expect(screen.getByText('Rows')).toBeVisible()
    expect(screen.getAllByText('users').length).toBeGreaterThan(0)
  })

  it('shows the table count for admin_db_schema', () => {
    renderTool(
      createTool('admin_db_schema', {}, { tables: [{ table: 'users' }] })
    )

    expect(screen.getByText('Database schema')).toBeVisible()
    expect(screen.getByText('Tables')).toBeVisible()
  })

  it('uses a readable title for list_operations and get_operation', () => {
    renderTool(createTool('list_operations', {}, { returned: 3 }))
    expect(screen.getByText('Operations')).toBeVisible()
    cleanup()
    renderTool(
      createTool('get_operation', {}, { operation_id: 'GET /api/user' })
    )
    expect(screen.getByText('Operation')).toBeVisible()
  })
})
