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
import { useStatus } from '@/hooks/use-status'

import { SettingsPage } from '../components/settings-page'
import type { OperationsSettings } from '../types'
import {
  OPERATIONS_DEFAULT_SECTION,
  getOperationsSectionContent,
  getOperationsSectionMeta,
} from './section-registry.tsx'

const defaultOperationsSettings: OperationsSettings = {
  DefaultCollapseSidebar: false,
  DemoSiteEnabled: false,
  SelfUseModeEnabled: false,
  WorkerUrl: '',
  WorkerValidKey: '',
  WorkerAllowHttpImageRequestEnabled: false,
  LogConsumeEnabled: false,
  'performance_setting.disk_cache_enabled': false,
  'performance_setting.disk_cache_threshold_mb': 10,
  'performance_setting.disk_cache_max_size_mb': 1024,
  'performance_setting.disk_cache_path': '',
  'performance_setting.monitor_enabled': false,
  'performance_setting.monitor_cpu_threshold': 90,
  'performance_setting.monitor_memory_threshold': 90,
  'performance_setting.monitor_disk_threshold': 95,
  'perf_metrics_setting.enabled': true,
  'perf_metrics_setting.flush_interval': 5,
  'perf_metrics_setting.bucket_time': 'hour',
  'perf_metrics_setting.retention_days': 0,
  'assistant_setting.enabled': true,
  'assistant_setting.user_enabled': false,
  'assistant_setting.user_source': 'direct',
  'assistant_setting.user_protocol': 'responses',
  'assistant_setting.user_base_url': 'https://api.openai.com/v1',
  'assistant_setting.user_api_key': '',
  'assistant_setting.user_model': 'gpt-4.1-mini',
  'assistant_setting.user_channel_id': 0,
  'assistant_setting.user_channel_model': '',
  'assistant_setting.user_context_window': 1000000,
  'assistant_setting.user_max_output_tokens': 128000,
  'assistant_setting.admin_enabled': false,
  'assistant_setting.admin_source': 'direct',
  'assistant_setting.admin_protocol': 'responses',
  'assistant_setting.admin_base_url': 'https://api.openai.com/v1',
  'assistant_setting.admin_api_key': '',
  'assistant_setting.admin_model': 'gpt-4.1-mini',
  'assistant_setting.admin_channel_id': 0,
  'assistant_setting.admin_channel_model': '',
  'assistant_setting.admin_context_window': 1000000,
  'assistant_setting.admin_max_output_tokens': 128000,
  'assistant_setting.system_prompt': '',
  'assistant_setting.request_timeout_seconds': 60,
  'assistant_setting.confirm_writes': true,
  'assistant_setting.admin_db_tools_enabled': true,
  'assistant_setting.user_api_key_configured': false,
  'assistant_setting.admin_api_key_configured': false,
}

export function OperationsSettings() {
  const { status } = useStatus()

  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/operations/$section'
      defaultSettings={defaultOperationsSettings}
      defaultSection={OPERATIONS_DEFAULT_SECTION}
      getSectionContent={getOperationsSectionContent}
      getSectionMeta={getOperationsSectionMeta}
      extraArgs={[
        status?.version as string | undefined,
        status?.start_time as number | null | undefined,
      ]}
      loadingMessage='Loading maintenance settings...'
    />
  )
}
