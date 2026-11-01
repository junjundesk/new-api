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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

type ActivityImagePreviewProps = {
  url: string
}

// 卡片背景图预览：填写后立刻显示缩略图，加载失败时给出可排查的提示。
// <img> 取图不经过 CORS（浏览器允许跨域渲染），失败通常是：图床防盗链（Referer）、
// HTTPS 页面引用 http 图片（混合内容）、或填的不是图片直链。因此这里固定带
// no-referrer 绕开最常见的 Referer 校验。
export function ActivityImagePreview(props: ActivityImagePreviewProps) {
  const { t } = useTranslation()
  // 记录加载失败的具体地址：地址一改就自动回到待加载状态，无需 effect 重置。
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  const url = props.url.trim()
  // 只预览合法的 http(s) 或站内绝对路径：非法值后端也会拒绝，这里避免渲染破图。
  if (!/^(https?:\/\/|\/)/.test(url)) return null

  if (window.location.protocol === 'https:' && /^http:\/\//i.test(url)) {
    return (
      <p className='text-destructive text-xs' role='alert'>
        {t('An https page cannot load http images.')}
      </p>
    )
  }

  if (failedUrl === url) {
    return (
      <div className='space-y-1 pt-1'>
        <p className='text-destructive text-xs' role='status'>
          {t('Image failed to load')}
        </p>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Common causes: hotlink protection, an http image on an https page, or a URL that is not a direct image link.'
          )}
        </p>
        <a
          href={url}
          target='_blank'
          rel='noreferrer noopener'
          className='text-xs underline underline-offset-2'
        >
          {t('Open in new window')}
        </a>
      </div>
    )
  }

  return (
    <div className='flex items-center gap-2 pt-1'>
      <img
        src={url}
        alt=''
        loading='lazy'
        referrerPolicy='no-referrer'
        className='border-border h-24 w-full max-w-xs rounded-md border object-cover'
        onError={() => setFailedUrl(url)}
      />
      <span className='text-muted-foreground text-xs'>{t('Preview')}</span>
    </div>
  )
}
