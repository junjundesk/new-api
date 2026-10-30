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
import { cleanup, render } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { TokenModelMapping } from '../../types'
import { ModelMappingEditor } from '../model-mapping-editor'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) => {
      let text = key
      for (const [name, value] of Object.entries(options ?? {})) {
        text = text.replaceAll(`{{${name}}}`, String(value))
      }
      return text
    },
  }),
}))

afterEach(() => {
  cleanup()
})

function renderEditor(value: TokenModelMapping[], models?: string[]) {
  return render(
    <ModelMappingEditor
      value={value}
      onChange={() => undefined}
      models={models}
    />
  )
}

const groupModels = ['gpt-4', 'gpt-4o', 'claude-sonnet-4-5']

describe('ModelMappingEditor shadowing hint', () => {
  it('warns that a group model name will be overridden without blocking the row', () => {
    const view = renderEditor(
      [{ source_model: 'gpt-4', target_model: 'gpt-4o' }],
      groupModels
    )

    const hint = view.getByText(/will be routed and billed to gpt-4o instead/)
    expect(hint.id).toBeTruthy()
    const sourceInput = view.getByLabelText('Source model')
    expect(sourceInput.getAttribute('aria-invalid')).not.toBe('true')
    expect(sourceInput.getAttribute('aria-describedby')).toBe(hint.id)
  })

  it('does not warn for an alias that only exists on the client side', () => {
    const view = renderEditor(
      [{ source_model: 'client-alias', target_model: 'gpt-4o' }],
      groupModels
    )

    expect(view.queryByText(/will be routed and billed/)).toBeNull()
  })

  it('does not warn for a self-mapping that only overrides reasoning effort', () => {
    const view = renderEditor(
      [{ source_model: 'gpt-4', target_model: 'gpt-4' }],
      groupModels
    )

    expect(view.queryByText(/will be routed and billed/)).toBeNull()
  })

  it('removes the warning when the source stops matching a group model', () => {
    const { rerender, getByText, queryByText } = renderEditor(
      [{ source_model: 'gpt-4', target_model: 'gpt-4o' }],
      groupModels
    )
    expect(
      getByText(/will be routed and billed to gpt-4o instead/)
    ).toBeTruthy()

    rerender(
      <ModelMappingEditor
        value={[{ source_model: 'client-alias', target_model: 'gpt-4o' }]}
        onChange={() => undefined}
        models={groupModels}
      />
    )

    expect(queryByText(/will be routed and billed/)).toBeNull()
  })
})
