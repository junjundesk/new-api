import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ModelBillingRatio } from '../components/model-billing-ratio'
import { ModelCard } from '../components/model-card'
import { PricingTable } from '../components/pricing-table'
import { ModelDetailsContent } from '../components/model-details'
import type { PricingModel } from '../types'

vi.mock('@lobehub/icons', () => ({}))
vi.mock('@lobehub/fluent-emoji', () => ({}))
vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))
vi.mock('@visactor/vchart', () => ({ default: class {}, registerAll: () => {} }))

const i18n = createInstance()
await i18n.init({ lng: 'en', resources: { en: { translation: {} } }, interpolation: { escapeValue: false } })
const model: PricingModel = {
  id: 1, model_name: 'public-model', quota_type: 0, model_ratio: 2, completion_ratio: 3,
  enable_groups: ['default'], group_ratio: { default: 2 },
  billing_ratios: { default: { min: 0.8, max: 1.2, source: 'channel_model' } },
}
const clients: QueryClient[] = []
function renderUI(element: React.ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { enabled: false, retry: false } } })
  clients.push(client)
  return render(<QueryClientProvider client={client}><I18nextProvider i18n={i18n}>{element}</I18nextProvider></QueryClientProvider>)
}
afterEach(() => { cleanup(); for (const client of clients.splice(0)) client.clear() })

describe('model billing ratio display', () => {
  it('opens explanations by keyboard and pointer without opening the model row', async () => {
    const user = userEvent.setup()
    const onRowClick = vi.fn()
    renderUI(<div onClick={onRowClick}><ModelBillingRatio model={model} selectedGroup='default' /></div>)
    const trigger = screen.getByRole('button', { name: 'Billing ratio 0.8–1.2× · Model-specific' })
    trigger.focus()
    await user.keyboard('{Enter}')
    expect(await screen.findByText('Available channels have different ratios. The price range includes fallback channels; the actual price depends on the channel used.')).toBeVisible()
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(trigger).toHaveAttribute('aria-expanded', 'false'))
    await user.click(trigger)
    expect(await screen.findByRole('dialog')).toBeVisible()
    expect(onRowClick).not.toHaveBeenCalled()
  })

  it('shows mixed same-price sources as one number and hides single ratios for all groups', async () => {
    const user = userEvent.setup()
    const mixed = { ...model, billing_ratios: { default: { min: 0, max: 0, source: 'mixed' as const } } }
    const { rerender } = renderUI(<ModelBillingRatio model={mixed} selectedGroup='default' />)
    await user.click(screen.getByRole('button', { name: 'Billing ratio 0× · By channel' }))
    expect(await screen.findByText('Channels use different ratio sources but currently have the same effective price.')).toBeVisible()
    expect(screen.queryByText('Available channels have different ratios. The price range includes fallback channels; the actual price depends on the channel used.')).not.toBeInTheDocument()
    rerender(<I18nextProvider i18n={i18n}><ModelBillingRatio model={mixed} selectedGroup='all' /></I18nextProvider>)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it.each(['channel_model', 'user_group', 'group'] as const)('shows the correct %s source', (source) => {
    const labels = { channel_model: 'Model-specific', user_group: 'User group offer', group: 'Group default' }
    renderUI(<ModelBillingRatio model={{ ...model, billing_ratios: { default: { min: 1, max: 1, source } } }} selectedGroup='default' />)
    expect(screen.getByRole('button', { name: `Billing ratio 1× · ${labels[source]}` })).toBeVisible()
  })

  it('renders matching selected-group ratio and price ranges in cards and tables', () => {
    const view = renderUI(<ModelCard model={model} onClick={vi.fn()} selectedGroup='default' />)
    expect(screen.getByRole('button', { name: 'Billing ratio 0.8–1.2× · Model-specific' })).toBeVisible()
    expect(screen.getAllByText((content) => content.startsWith('$3.2–$4.8')).length).toBeGreaterThan(0)
    view.unmount()
    renderUI(<PricingTable models={[model]} selectedGroup='default' />)
    expect(screen.getByRole('button', { name: 'Billing ratio 0.8–1.2× · Model-specific' })).toBeVisible()
    expect(screen.getByText('$3.2', { exact: false })).toHaveTextContent('$3.2–$4.8')
  })

  it('shows inherited selected-group pricing consistently in details and tier previews', () => {
    const dynamic = { ...model, billing_mode: 'tiered_expr', billing_expr: 'tier("standard", p * 4 + c * 12)' }
    renderUI(<ModelDetailsContent model={dynamic} selectedGroup='default' groupRatio={{ default: 2 }} usableGroup={{ default: { desc: 'Default', ratio: 2 } }} autoGroups={[]} endpointMap={{}} tokenUnit='M' priceRate={1} usdExchangeRate={1} />)
    expect(screen.getByRole('button', { name: 'Billing ratio 0.8–1.2× · Model-specific' })).toBeVisible()
    const pricing = screen.getByText('Selected group price').closest('section')!
    expect(within(pricing).getAllByText((content) => content.startsWith('$9.6–$14.4')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('$9.6–$14.4').length).toBeGreaterThan(1)
  })
})
