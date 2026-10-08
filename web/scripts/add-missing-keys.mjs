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
import fs from 'node:fs/promises'
import path from 'node:path'

// Adds missing performance-page keys to every locale without re-serializing the
// files, so unrelated formatting and ordering stay untouched. Keys are inserted
// next to their alphabetical neighbours; run "bun run i18n:sync" afterwards for
// the canonical ordering.
const LOCALES_DIR = path.resolve('src/i18n/locales')
const LOCALES = ['en', 'zh']

const NEW_KEYS = {
  Performance: {
    en: 'Performance',
    zh: '性能',
  },
  'Live availability and latency for the groups your account can use.': {
    en: 'Live availability and latency for the groups your account can use.',
    zh: '当前账号可用分组的实时可用率与延迟。',
  },
  'Updated {{time}}': {
    en: 'Updated {{time}}',
    zh: '更新于 {{time}}',
  },
  'Auto-refreshes every minute': {
    en: 'Auto-refreshes every minute',
    zh: '每分钟自动刷新',
  },
  'Time range': {
    en: 'Time range',
    zh: '时间范围',
  },
  'Last 24 hours': {
    en: 'Last 24 hours',
    zh: '最近 24 小时',
  },
  'Last 7 days': {
    en: 'Last 7 days',
    zh: '最近 7 天',
  },
  Refresh: {
    en: 'Refresh',
    zh: '刷新',
  },
  'Sort groups': {
    en: 'Sort groups',
    zh: '排序分组',
  },
  'Custom group order': {
    en: 'Custom group order',
    zh: '自定义分组顺序',
  },
  'Request volume': {
    en: 'Request volume',
    zh: '请求量',
  },
  'Observed multiplier: high to low': {
    en: 'Observed multiplier: high to low',
    zh: '实测倍率：从高到低',
  },
  'Observed multiplier: low to high': {
    en: 'Observed multiplier: low to high',
    zh: '实测倍率：从低到高',
  },
  'Effective multiplier: high to low': {
    en: 'Effective multiplier: high to low',
    zh: '实际倍率：从高到低',
  },
  'Effective multiplier: low to high': {
    en: 'Effective multiplier: low to high',
    zh: '实际倍率：从低到高',
  },
  'Group name': {
    en: 'Group name',
    zh: '分组名称',
  },
  'Coding cache: high to low': {
    en: 'Coding cache: high to low',
    zh: 'Coding 缓存：从高到低',
  },
  'Success rate: high to low': {
    en: 'Success rate: high to low',
    zh: '成功率：从高到低',
  },
  'Latency: low to high': {
    en: 'Latency: low to high',
    zh: '延迟：从低到高',
  },
  'Throughput: high to low': {
    en: 'Throughput: high to low',
    zh: '吞吐量：从高到低',
  },
  'Filter groups by status': {
    en: 'Filter groups by status',
    zh: '按状态筛选分组',
  },
  Operational: {
    en: 'Operational',
    zh: '运行中',
  },
  Degraded: {
    en: 'Degraded',
    zh: '波动',
  },
  Unavailable: {
    en: 'Unavailable',
    zh: '异常',
  },
  'No data': {
    en: 'No data',
    zh: '暂无数据',
  },
  '{{shown}} of {{count}} groups': {
    en: '{{shown}} of {{count}} groups',
    zh: '{{shown}}/{{count}} 个分组',
  },
  '{{count}} groups': {
    en: '{{count}} groups',
    zh: '{{count}} 个分组',
  },
  'Use Move up and Move down to save your group order on this device. Filters keep hidden groups in place.':
    {
      en: 'Use Move up and Move down to save your group order on this device. Filters keep hidden groups in place.',
      zh: '使用上移、下移调整顺序，并保存在此设备。筛选时隐藏的分组保持原位置。',
    },
  'Move up': {
    en: 'Move up',
    zh: '上移',
  },
  'Move down': {
    en: 'Move down',
    zh: '下移',
  },
  'Move {{group}} up': {
    en: 'Move {{group}} up',
    zh: '将 {{group}} 上移',
  },
  'Move {{group}} down': {
    en: 'Move {{group}} down',
    zh: '将 {{group}} 下移',
  },
  'Group ratio': {
    en: 'Group ratio',
    zh: '分组倍率',
  },
  'User Group': {
    en: 'User Group',
    zh: '用户分组',
  },
  Auto: {
    en: 'Auto',
    zh: '自动',
  },
  'Cache hit': {
    en: 'Cache hit',
    zh: '缓存命中',
  },
  'Coding cache': {
    en: 'Coding cache',
    zh: 'Coding 缓存',
  },
  'Observed input multiplier': {
    en: 'Observed input multiplier',
    zh: '实测输入倍率',
  },
  'Reference-adjusted multiplier': {
    en: 'Reference-adjusted multiplier',
    zh: '参考调整倍率',
  },
  'Pricing coverage is insufficient': {
    en: 'Pricing coverage is insufficient',
    zh: '价格覆盖不足',
  },
  'Cost coverage: {{coverage}}%': {
    en: 'Cost coverage: {{coverage}}%',
    zh: '成本覆盖：{{coverage}}%',
  },
  Latency: {
    en: 'Latency',
    zh: '延迟',
  },
  Availability: {
    en: 'Availability',
    zh: '可用性',
  },
  '{{success}}/{{total}} requests succeeded': {
    en: '{{success}}/{{total}} requests succeeded',
    zh: '{{success}}/{{total}} 次请求成功',
  },
  'No requests in this window': {
    en: 'No requests in this window',
    zh: '该时间窗内无请求',
  },
  '{{count}} models with traffic': {
    en: '{{count}} models with traffic',
    zh: '{{count}} 个模型有流量',
  },
  'View models': {
    en: 'View models',
    zh: '查看模型',
  },
  Past: {
    en: 'Past',
    zh: '过去',
  },
  Now: {
    en: 'Now',
    zh: '现在',
  },
  '{{count}} requests': {
    en: '{{count}} requests',
    zh: '{{count}} 次请求',
  },
  'Cache metrics explained': {
    en: 'Cache metrics explained',
    zh: '缓存统计说明',
  },
  'Cache hit is cached input tokens divided by total input tokens, summed across successful requests in this group. Output tokens are excluded.':
    {
      en: 'Cache hit is cached input tokens divided by total input tokens, summed across successful requests in this group. Output tokens are excluded.',
      zh: '缓存命中 = 缓存读取 token 总数 ÷ 总输入 token 数，按本分组的成功调用汇总，不包含输出 token。',
    },
  'Coding cache uses the same calculation, only for Codex, Claude Code, Pi, OpenCode, OMP, ZCode, DeepSeek Harness (DSH), and Open Design clients.':
    {
      en: 'Coding cache uses the same calculation, only for Codex, Claude Code, Pi, OpenCode, OMP, ZCode, DeepSeek Harness (DSH), and Open Design clients.',
      zh: 'Coding 缓存采用相同公式，仅统计 Codex、Claude Code、Pi、OpenCode、OMP、ZCode、DeepSeek Harness (DSH) 和 Open Design 客户端。',
    },
  'Observed multiplier = input cost ÷ the 0% cache reference cost. Reference-adjusted multiplier uses your selected cache scenario instead. Both keep cache-write costs fixed and require 90% input cost coverage.':
    {
      en: 'Observed multiplier = input cost ÷ the 0% cache reference cost. Reference-adjusted multiplier uses your selected cache scenario instead. Both keep cache-write costs fixed and require 90% input cost coverage.',
      zh: '观测倍率 = 输入成本 ÷ 缓存 0% 的参考成本；参考调整倍率使用你选择的缓存场景作为分母。两者均保持写入费用不变，输入成本覆盖达到 90% 才展示。',
    },
  'Only valid upstream usage is included. No data is shown as —; a measured zero is 0%. Statistics cover all users in the group.':
    {
      en: 'Only valid upstream usage is included. No data is shown as —; a measured zero is 0%. Statistics cover all users in the group.',
      zh: '仅纳入有效的上游用量。无数据时显示 —，真实零命中显示 0%。统计覆盖本分组的所有用户。',
    },
  'Reference cache rate': {
    en: 'Reference cache rate',
    zh: '参考缓存率',
  },
  'Reference cache: {{rate}}%': {
    en: 'Reference cache: {{rate}}%',
    zh: '参考缓存率：{{rate}}%',
  },
  'Reference cache rate (%)': {
    en: 'Reference cache rate (%)',
    zh: '参考缓存率（%）',
  },
  '90% is an adjustable multi-turn coding scenario, not a guaranteed official cache rate. OpenAI documents examples above 90%, with results depending on the workload.':
    {
      en: '90% is an adjustable multi-turn coding scenario, not a guaranteed official cache rate. OpenAI documents examples above 90%, with results depending on the workload.',
      zh: '90% 是可调整的多轮 Coding 参考场景，不是官方保证的缓存率。OpenAI 文档列举了超过 90% 的案例，实际效果取决于工作负载。',
    },
  'Changing this reference only changes the estimate, not your billing.': {
    en: 'Changing this reference only changes the estimate, not your billing.',
    zh: '调整参考值只影响估算结果，不会改变计费。',
  },
  'The reference changes cache reads for non-write input. Actual cache-write costs stay fixed; Coding cache percentages include writes in total input.':
    {
      en: 'The reference changes cache reads for non-write input. Actual cache-write costs stay fixed; Coding cache percentages include writes in total input.',
      zh: '参考缓存率仅调整非写入输入的缓存读取；缓存写入费用保持不变。Coding 缓存率的输入总量包含写入。',
    },
  'Input price estimate': {
    en: 'Input price estimate',
    zh: '输入价格估算',
  },
  'USD per 1M input tokens': {
    en: 'USD per 1M input tokens',
    zh: '美元 / 百万输入 token',
  },
  'Input price at 0% reference cache': {
    en: 'Input price at 0% reference cache',
    zh: '参考缓存 0% 时的输入价',
  },
  'Input price at 100% reference cache': {
    en: 'Input price at 100% reference cache',
    zh: '参考缓存 100% 时的输入价',
  },
  'Observed input price': {
    en: 'Observed input price',
    zh: '观测输入单价',
  },
  'Reference input price': {
    en: 'Reference input price',
    zh: '参考输入单价',
  },
  'Reference cache {{rate}}% · all Coding cache {{observed}}% · input cost coverage {{coverage}}%':
    {
      en: 'Reference cache {{rate}}% · all Coding cache {{observed}}% · input cost coverage {{coverage}}%',
      zh: '参考缓存 {{rate}}% · 全部 Coding 缓存 {{observed}}% · 输入成本覆盖 {{coverage}}%',
    },
  'Prices describe the observed subset. Multipliers require 90% input cost coverage for the group or model being shown.':
    {
      en: 'Prices describe the observed subset. Multipliers require 90% input cost coverage for the group or model being shown.',
      zh: '单价仅描述已观测的样本。分组或模型的输入成本覆盖达到 90% 后，才展示对应倍率。',
    },
  'Observed input cost uses each request’s settlement prices and multiplier. Reference cost uses the same pricing rules before the group multiplier; these are site prices, not verified official prices.':
    {
      en: 'Observed input cost uses each request’s settlement prices and multiplier. Reference cost uses the same pricing rules before the group multiplier; these are site prices, not verified official prices.',
      zh: '观测成本采用每次请求结算时的价格与倍率。参考成本沿用同一计价规则、不含分组倍率；基准来自站点配置，未经官网价格核验。',
    },
  'Input costs include regular input, cache reads, and cache writes. Output, tools, and recharge discounts are excluded. Historical usage without cost observations and unsupported pricing reduce coverage.':
    {
      en: 'Input costs include regular input, cache reads, and cache writes. Output, tools, and recharge discounts are excluded. Historical usage without cost observations and unsupported pricing reduce coverage.',
      zh: '输入成本包含普通输入、缓存读取和缓存写入，不含输出、工具费用与充值优惠。未采集成本的历史用量及不支持的计价规则会降低覆盖率。',
    },
  'Costs are calculated before quota rounding. Nonlinear, cache-dependent, media, and per-request pricing may remain unobserved.':
    {
      en: 'Costs are calculated before quota rounding. Nonlinear, cache-dependent, media, and per-request pricing may remain unobserved.',
      zh: '成本在整数额度舍入前计算。非线性、依赖缓存条件、媒体及按次计费等规则可能没有成本观测。',
    },
  'No valid Coding input was observed in this window.': {
    en: 'No valid Coding input was observed in this window.',
    zh: '此时间窗口内没有有效的 Coding 输入观测。',
  },
  'Model breakdown': {
    en: 'Model breakdown',
    zh: '模型明细',
  },
  'Per-model availability within this group': {
    en: 'Per-model availability within this group',
    zh: '该分组内按模型的可用率',
  },
  Model: {
    en: 'Model',
    zh: '模型',
  },
  Requests: {
    en: 'Requests',
    zh: '请求数',
  },
  'Success rate': {
    en: 'Success rate',
    zh: '成功率',
  },
  'Average TTFT': {
    en: 'Average TTFT',
    zh: '平均首 Token 延迟',
  },
  'Average latency': {
    en: 'Average latency',
    zh: '平均延迟',
  },
  'No model traffic in this group yet.': {
    en: 'No model traffic in this group yet.',
    zh: '该分组暂无模型流量。',
  },
  'Failed to load performance data.': {
    en: 'Failed to load performance data.',
    zh: '性能数据加载失败。',
  },
  Retry: {
    en: 'Retry',
    zh: '重试',
  },
  'You have no usable groups yet.': {
    en: 'You have no usable groups yet.',
    zh: '当前账号暂无可用分组。',
  },
  'Last hour': {
    en: 'Last hour',
    zh: '最近 1 小时',
  },
}

// Values that exist in every locale but still equal the English key.
// Populated per locale when a key needs a real translation instead of a copy
// of the English source string.
const UPDATED_VALUES = {}

function escapeJson(value) {
  return JSON.stringify(value)
}

function keyLineIndex(lines, key, from, to) {
  const target = JSON.parse(escapeJson(key))
  for (let i = from; i < to; i += 1) {
    const match = lines[i].match(/^\s*"((?:[^"\\]|\\.)*)"\s*:/)
    if (!match) continue
    if (JSON.parse('"' + match[1] + '"') === target) return i
  }
  return -1
}

async function insertMissingKeys(locale) {
  const file = path.join(LOCALES_DIR, locale + '.json')
  const text = await fs.readFile(file, 'utf8')
  const parsed = JSON.parse(text)
  const existing = parsed.translation ?? {}
  const pending = Object.keys(NEW_KEYS)
    .filter((key) => NEW_KEYS[key][locale] && existing[key] === undefined)
    .sort((a, b) => a.localeCompare(b, 'en'))
  const updatesNeeded = Object.keys(UPDATED_VALUES).some(
    (key) => UPDATED_VALUES[key][locale] && existing[key] === key
  )
  if (pending.length === 0 && !updatesNeeded) return 0

  const lines = text.split('\n')
  const translationStart = lines.findIndex((line) =>
    /"translation"\s*:\s*\{/.test(line)
  )
  if (translationStart < 0)
    throw new Error('translation object not found in ' + file)
  let end = lines.length - 1
  for (let i = translationStart + 1; i < lines.length; i += 1) {
    if (/^\s*\}\s*,?\s*$/.test(lines[i])) {
      end = i
      break
    }
  }

  for (const key of pending) {
    let insertAt = keyLineIndex(lines, key, translationStart + 1, end)
    if (insertAt < 0) {
      // Keep the trailing key unprefixed by inserting after the last key line.
      let lastKeyLine = -1
      for (let i = end - 1; i > translationStart; i -= 1) {
        if (/^\s*"(?:[^"\\]|\\.)*"\s*:/.test(lines[i])) {
          lastKeyLine = i
          break
        }
      }
      if (lastKeyLine < 0)
        throw new Error('no key lines before the end of translation in ' + file)
      // The last key line has no trailing comma: insert before it instead.
      lines.splice(
        lastKeyLine,
        0,
        '    ' +
          escapeJson(key) +
          ': ' +
          escapeJson(NEW_KEYS[key][locale]) +
          ','
      )
      end += 1
      continue
    }
    lines.splice(
      insertAt,
      0,
      '    ' + escapeJson(key) + ': ' + escapeJson(NEW_KEYS[key][locale]) + ','
    )
    end += 1
  }

  for (const key of Object.keys(UPDATED_VALUES)) {
    const value = UPDATED_VALUES[key][locale]
    if (!value || existing[key] !== key) continue
    const at = keyLineIndex(lines, key, translationStart + 1, lines.length - 1)
    if (at < 0) continue
    const comma = lines[at].trimEnd().endsWith(',') ? ',' : ''
    lines[at] = '    ' + escapeJson(key) + ': ' + escapeJson(value) + comma
  }

  await fs.writeFile(file, lines.join('\n'), 'utf8')
  return pending.length
}

async function main() {
  const summary = []
  for (const locale of LOCALES) {
    const added = await insertMissingKeys(locale)
    summary.push(locale + ': ' + added)
  }
  console.log('add-missing-keys done -> ' + summary.join(', '))
}

await main()
