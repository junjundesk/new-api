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
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it } from 'vitest'

import { ActivityImagePreview } from '../activity-image-preview'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Preview: 'Preview',
        'Image failed to load': 'Image failed to load',
      },
    },
  },
  interpolation: { escapeValue: false },
})

function renderPreview(url: string) {
  return render(
    <I18nextProvider i18n={i18n}>
      <ActivityImagePreview url={url} />
    </I18nextProvider>
  )
}

afterEach(cleanup)

describe('activity image preview', () => {
  // 缩略图是装饰性图片（alt=''，旁边有 "Preview" 文案），因此不暴露 img role。
  it('renders nothing when the url is empty or only whitespace', () => {
    const { container } = renderPreview('   ')
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing for a url that is neither http(s) nor site-relative', () => {
    const { container } = renderPreview('javascript:alert(1)')
    expect(container).toBeEmptyDOMElement()
  })

  it('renders the thumbnail with the given url', () => {
    const { container } = renderPreview('https://cdn.example.com/banner.png')
    expect(container.querySelector('img')).toHaveAttribute(
      'src',
      'https://cdn.example.com/banner.png'
    )
    expect(screen.getByText('Preview')).toBeVisible()
  })

  it('shows a fallback message when the image fails to load', () => {
    const { container } = renderPreview('/images/missing.png')
    fireEvent.error(container.querySelector('img') as HTMLImageElement)
    expect(screen.getByRole('status')).toHaveTextContent('Image failed to load')
    expect(container.querySelector('img')).toBeNull()
  })

  it('retries the thumbnail after the url changes', () => {
    const { container, rerender } = renderPreview('/images/missing.png')
    fireEvent.error(container.querySelector('img') as HTMLImageElement)
    expect(screen.getByRole('status')).toBeVisible()

    rerender(
      <I18nextProvider i18n={i18n}>
        <ActivityImagePreview url='/images/banner.png' />
      </I18nextProvider>
    )
    expect(container.querySelector('img')).toHaveAttribute(
      'src',
      '/images/banner.png'
    )
  })
})
