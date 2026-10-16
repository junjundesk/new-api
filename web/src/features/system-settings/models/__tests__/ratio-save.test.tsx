import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { toast } from 'sonner'

import { useSaveOptions } from '../../hooks/use-save-options'

await i18next.init({ lng: 'en', resources: { en: { translation: {} } } })

function SaveHarness(props: {
  entries: Array<{ key: string; value: string | number | boolean }>
}) {
  const { saveOptions, isSaving } = useSaveOptions()
  return (
    <button
      type='button'
      onClick={() => {
        void saveOptions(props.entries)
      }}
    >
      {isSaving ? 'saving' : 'save'}
    </button>
  )
}

function renderHarness(
  entries: Array<{ key: string; value: string | number | boolean }>
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
  render(
    <QueryClientProvider client={queryClient}>
      <SaveHarness entries={entries} />
    </QueryClientProvider>
  )
  return { queryClient, invalidateSpy }
}

describe('unified option saving', () => {
  let putSpy: ReturnType<typeof vi.spyOn>
  let successSpy: ReturnType<typeof vi.spyOn>
  let infoSpy: ReturnType<typeof vi.spyOn>
  let errorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    putSpy = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true, message: '' } })
    successSpy = vi.spyOn(toast, 'success').mockImplementation(() => '')
    infoSpy = vi.spyOn(toast, 'info').mockImplementation(() => '')
    errorSpy = vi.spyOn(toast, 'error').mockImplementation(() => '')
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('writes every changed option once and reports a single outcome', async () => {
    const { invalidateSpy } = renderHarness([
      { key: 'GroupRatio', value: '{"vip":0.5}' },
      { key: 'UserUsableGroups', value: '{"vip":"VIP"}' },
    ])

    fireEvent.click(screen.getByRole('button', { name: 'save' }))

    await waitFor(() => expect(putSpy).toHaveBeenCalledTimes(2))
    expect(putSpy).toHaveBeenNthCalledWith(1, '/api/option/', {
      key: 'GroupRatio',
      value: '{"vip":0.5}',
    })
    expect(putSpy).toHaveBeenNthCalledWith(2, '/api/option/', {
      key: 'UserUsableGroups',
      value: '{"vip":"VIP"}',
    })
    expect(successSpy).toHaveBeenCalledTimes(1)
    expect(errorSpy).not.toHaveBeenCalled()
    await waitFor(() =>
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['system-options'] })
    )
    // The options refresh happens once, after every write settled.
    expect(
      invalidateSpy.mock.calls.filter(
        (call) =>
          JSON.stringify(call[0]) === JSON.stringify({ queryKey: ['system-options'] })
      )
    ).toHaveLength(1)
  })

  it('reports no changes without sending a request', async () => {
    renderHarness([])

    fireEvent.click(screen.getByRole('button', { name: 'save' }))

    await waitFor(() => expect(infoSpy).toHaveBeenCalledTimes(1))
    expect(putSpy).not.toHaveBeenCalled()
    expect(successSpy).not.toHaveBeenCalled()
  })

  it('ignores a second save while the first one is still running', async () => {
    const pending: Array<(value: { data: { success: boolean } }) => void> = []
    putSpy.mockImplementation(
      () =>
        new Promise((resolve) => {
          pending.push(resolve)
        })
    )

    renderHarness([{ key: 'GroupRatio', value: '{"vip":0.5}' }])

    const button = screen.getByRole('button', { name: 'save' })
    fireEvent.click(button)
    await waitFor(() => expect(screen.getByText('saving')).toBeTruthy())
    fireEvent.click(screen.getByText('saving'))
    expect(putSpy).toHaveBeenCalledTimes(1)

    pending[0]?.({ data: { success: true } })
    await waitFor(() => expect(successSpy).toHaveBeenCalledTimes(1))
    expect(putSpy).toHaveBeenCalledTimes(1)
    expect(errorSpy).not.toHaveBeenCalled()
  })

  it('reports one error and refreshes when a write fails', async () => {
    putSpy.mockResolvedValueOnce({
      data: { success: false, message: '分组倍率设置失败' },
    })
    const { invalidateSpy } = renderHarness([
      { key: 'GroupRatio', value: '{"vip":-1}' },
    ])

    fireEvent.click(screen.getByRole('button', { name: 'save' }))

    await waitFor(() => expect(errorSpy).toHaveBeenCalledTimes(1))
    expect(successSpy).not.toHaveBeenCalled()
    expect(errorSpy.mock.calls[0]?.[0]).toBe('分组倍率设置失败')
    await waitFor(() =>
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['system-options'] })
    )
  })
})
