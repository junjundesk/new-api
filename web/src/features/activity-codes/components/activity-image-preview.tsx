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

// 卡片背景图预览：填写后立刻显示缩略图，加载失败给出提示，
// 避免管理员填了死链却以为已经生效；空值不渲染任何节点。
export function ActivityImagePreview(props: ActivityImagePreviewProps) {
  const { t } = useTranslation()
  // 记录加载失败的具体地址：地址一改就自动回到待加载状态，无需 effect 重置。
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  // 只预览合法的 http(s) 或站内绝对路径：非法值后端也会拒绝，这里避免渲染出无意义的破图。
  const url = props.url.trim()
  if (!/^(https?:\/\/|\/)/.test(url)) return null

  if (failedUrl === url) {
    return (
      <p className='text-muted-foreground text-xs' role='status'>
        {t('Image failed to load')}
      </p>
    )
  }

  return (
    <div className='flex items-center gap-2'>
      <img
        src={url}
        alt=''
        className='border-border h-24 w-full max-w-xs rounded-md border object-cover'
        onError={() => setFailedUrl(url)}
      />
      <span className='text-muted-foreground text-xs'>{t('Preview')}</span>
    </div>
  )
}
