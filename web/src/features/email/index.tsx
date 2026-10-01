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
import { Loader2, Mail, Search, Send, X } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
import { searchUsers } from '@/features/users/api'

import { sendAdminEmail } from './api'
import type { EmailRecipient } from './types'

type TargetMode = 'selected' | 'all'

export function Email() {
  const { t } = useTranslation()
  const [mode, setMode] = useState<TargetMode>('selected')
  const [keyword, setKeyword] = useState('')
  const [results, setResults] = useState<EmailRecipient[]>([])
  const [searching, setSearching] = useState(false)
  const [selected, setSelected] = useState<EmailRecipient[]>([])
  const [subject, setSubject] = useState('')
  const [content, setContent] = useState('')
  const [sending, setSending] = useState(false)

  const selectedIds = useMemo(
    () => new Set(selected.map((u) => u.id)),
    [selected]
  )

  const handleSearch = useCallback(async () => {
    const trimmed = keyword.trim()
    if (!trimmed) {
      toast.error(t('Enter a username, email or user ID to search'))
      return
    }
    setSearching(true)
    try {
      const res = await searchUsers({ keyword: trimmed, p: 1, page_size: 20 })
      setResults((res.data?.items ?? []) as EmailRecipient[])
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t('Search failed'))
    } finally {
      setSearching(false)
    }
  }, [keyword, t])

  const toggleRecipient = (user: EmailRecipient) => {
    setSelected((prev) =>
      prev.some((u) => u.id === user.id)
        ? prev.filter((u) => u.id !== user.id)
        : [...prev, user]
    )
  }

  const resetForm = () => {
    setSubject('')
    setContent('')
    setSelected([])
    setResults([])
    setKeyword('')
  }

  const handleSend = async () => {
    if (!subject.trim()) {
      toast.error(t('Email subject is required'))
      return
    }
    if (!content.trim()) {
      toast.error(t('Email content is required'))
      return
    }
    if (mode === 'selected' && selected.length === 0) {
      toast.error(t('Select at least one recipient'))
      return
    }

    setSending(true)
    try {
      const res = await sendAdminEmail({
        all_users: mode === 'all',
        user_ids: mode === 'selected' ? selected.map((u) => u.id) : undefined,
        subject: subject.trim(),
        content,
      })
      if (!res.success) {
        toast.error(res.message || t('Failed to send email'))
        return
      }

      const data = res.data
      const skipped = data?.skipped ?? 0
      if (data?.async) {
        toast.success(
          t('Broadcast queued for {{count}} recipients', {
            count: data.queued,
          })
        )
      } else {
        toast.success(
          t('Email sent to {{sent}} recipients ({{failed}} failed)', {
            sent: data?.sent ?? 0,
            failed: data?.failed ?? 0,
          })
        )
      }
      if (skipped > 0) {
        toast.warning(
          t('{{count}} users were skipped because they have no email address', {
            count: skipped,
          })
        )
      }
      resetForm()
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t('Failed to send email'))
    } finally {
      setSending(false)
    }
  }

  const canSend =
    subject.trim().length > 0 &&
    content.trim().length > 0 &&
    (mode === 'all' || selected.length > 0)

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Send Email')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto w-full max-w-3xl space-y-5'>
          {/* Recipients */}
          <div className='space-y-3 rounded-lg border p-4'>
            <div className='flex items-center gap-2'>
              <Mail className='text-muted-foreground h-4 w-4' />
              <h3 className='text-sm font-medium'>{t('Recipients')}</h3>
            </div>

            <div className='flex flex-wrap gap-4'>
              <label className='flex cursor-pointer items-center gap-2 text-sm'>
                <Checkbox
                  checked={mode === 'selected'}
                  onCheckedChange={() => setMode('selected')}
                />
                {t('Selected users')}
              </label>
              <label className='flex cursor-pointer items-center gap-2 text-sm'>
                <Checkbox
                  checked={mode === 'all'}
                  onCheckedChange={() => setMode('all')}
                />
                {t('All users')}
              </label>
            </div>

            {mode === 'all' ? (
              <Alert>
                <AlertDescription>
                  {t(
                    'This message will be sent to every user that has an email address. Delivery runs in the background.'
                  )}
                </AlertDescription>
              </Alert>
            ) : (
              <div className='space-y-3'>
                <div className='flex gap-2'>
                  <Input
                    value={keyword}
                    onChange={(e) => setKeyword(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') void handleSearch()
                    }}
                    placeholder={t(
                      'Search by username, email or user ID'
                    )}
                  />
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => void handleSearch()}
                    disabled={searching}
                  >
                    {searching ? (
                      <Loader2 className='h-4 w-4 animate-spin' />
                    ) : (
                      <Search className='h-4 w-4' />
                    )}
                    {t('Search')}
                  </Button>
                </div>

                {/* Selected chips */}
                {selected.length > 0 && (
                  <div className='flex flex-wrap gap-1.5'>
                    {selected.map((user) => (
                      <Badge key={user.id} variant='secondary' className='gap-1'>
                        {user.username}
                        <button
                          type='button'
                          onClick={() => toggleRecipient(user)}
                          aria-label={t('Remove')}
                        >
                          <X className='h-3 w-3' />
                        </button>
                      </Badge>
                    ))}
                  </div>
                )}

                {/* Search results */}
                {results.length > 0 && (
                  <ScrollArea className='h-48 rounded-md border'>
                    <div className='divide-y'>
                      {results.map((user) => {
                        const checked = selectedIds.has(user.id)
                        const noEmail = !user.email?.trim()
                        return (
                          <label
                            key={user.id}
                            className='hover:bg-muted/50 flex cursor-pointer items-center gap-3 px-3 py-2 text-sm'
                          >
                            <Checkbox
                              checked={checked}
                              disabled={noEmail}
                              onCheckedChange={() => toggleRecipient(user)}
                            />
                            <span className='min-w-0 flex-1 truncate'>
                              {user.display_name || user.username}
                            </span>
                            <span
                              className={
                                noEmail
                                  ? 'text-destructive shrink-0 text-xs'
                                  : 'text-muted-foreground shrink-0 text-xs'
                              }
                            >
                              {noEmail ? t('No email') : user.email}
                            </span>
                          </label>
                        )
                      })}
                    </div>
                  </ScrollArea>
                )}
              </div>
            )}
          </div>

          {/* Message */}
          <div className='space-y-3 rounded-lg border p-4'>
            <div className='space-y-2'>
              <Label htmlFor='email-subject'>{t('Subject')}</Label>
              <Input
                id='email-subject'
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
              />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='email-content'>{t('Content')}</Label>
              <Textarea
                id='email-content'
                rows={10}
                value={content}
                onChange={(e) => setContent(e.target.value)}
                placeholder={t('HTML is supported')}
              />
            </div>
          </div>

          <div className='flex justify-end gap-2'>
            <Button type='button' variant='outline' onClick={resetForm}>
              {t('Reset')}
            </Button>
            <Button
              type='button'
              onClick={() => void handleSend()}
              disabled={!canSend || sending}
            >
              {sending ? (
                <Loader2 className='h-4 w-4 animate-spin' />
              ) : (
                <Send className='h-4 w-4' />
              )}
              {sending ? t('Sending...') : t('Send')}
            </Button>
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
