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
  "Yesterday": {
    en: "Yesterday",
    zh: "昨天",
  },
  "Last 7 Days": {
    en: "Last 7 Days",
    zh: "最近 7 天",
  },
  "Last 30 Days": {
    en: "Last 30 Days",
    zh: "最近 30 天",
  },
  "User Consumption Details": {
    en: "User Consumption Details",
    zh: "用户消费明细",
  },
  "Consumption by user, group and model for the selected time range.": {
    en: "Consumption by user, group and model for the selected time range.",
    zh: "所选时间范围内，各用户在各分组、各模型上的消费明细。",
  },
  "Total Requests": {
    en: "Total Requests",
    zh: "总请求数",
  },
  "Active Users": {
    en: "Active Users",
    zh: "活跃用户",
  },
  "Filter by model": {
    en: "Filter by model",
    zh: "按模型筛选",
  },
  "No consumption in this range": {
    en: "No consumption in this range",
    zh: "该时间段内无消费数据",
  },
  "Only the top {{count}} rows by cost are shown. Narrow the time range for complete data.": {
    en: "Only the top {{count}} rows by cost are shown. Narrow the time range for complete data.",
    zh: "仅显示消费最高的 {{count}} 条记录，请缩小时间范围以查看完整数据。",
  },
  "Usage data collection is disabled; enable data export to collect this data.": {
    en: "Usage data collection is disabled; enable data export to collect this data.",
    zh: "用量采集已关闭，请在系统设置中开启数据导出后再查看。",
  },
  "Usage data is aggregated every {{count}} minutes.": {
    en: "Usage data is aggregated every {{count}} minutes.",
    zh: "用量数据每 {{count}} 分钟聚合一次。",
  },
  "Remaining": {
    en: "Remaining",
    zh: "剩余",
  },
  "Limited-time group access granted by an activity code": {
    en: "Limited-time group access granted by an activity code",
    zh: "活动码授予的限时分组权限",
  },
  "Redeem {{name}}": {
    en: "Redeem {{name}}",
    zh: "兑换 {{name}}",
  },
  "Original price before the activity code discount ({{ratio}}x group ratio)": {
    en: "Original price before the activity code discount ({{ratio}}x group ratio)",
    zh: "活动码优惠前的原价（{{ratio}}x 分组倍率）",
  },
  "User Logs": {
    en: "User Logs",
    zh: "用户日志",
  },
  "Content": {
    en: "Content",
    zh: "内容",
  },
  "IP Address": {
    en: "IP Address",
    zh: "IP 地址",
  },
  "Request Source": {
    en: "Request Source",
    zh: "请求来源",
  },
  "Type": {
    en: "Type",
    zh: "类型",
  },
  "No user logs yet": {
    en: "No user logs yet",
    zh: "暂无用户日志",
  },
  "Account activity such as sign-in, registration, API keys and top-ups appears here.": {
    en: "Account activity such as sign-in, registration, API keys and top-ups appears here.",
    zh: "登录、注册、密钥、充值等账户操作记录会显示在这里。",
  },
  "Filter by username...": {
    en: "Filter by username...",
    zh: "按用户名筛选…",
  },
  "Created API key {{name}}": {
    en: "Created API key {{name}}",
    zh: "新建密钥 {{name}}",
  },
  "Updated API key {{name}}": {
    en: "Updated API key {{name}}",
    zh: "修改密钥 {{name}}",
  },
  "Deleted API key {{name}} (ID: {{id}})": {
    en: "Deleted API key {{name}} (ID: {{id}})",
    zh: "删除密钥 {{name}}（ID: {{id}}）",
  },
  "Deleted {{count}} API keys": {
    en: "Deleted {{count}} API keys",
    zh: "批量删除 {{count}} 个密钥",
  },
  "Enabled API key {{name}}": {
    en: "Enabled API key {{name}}",
    zh: "启用密钥 {{name}}",
  },
  "Disabled API key {{name}}": {
    en: "Disabled API key {{name}}",
    zh: "停用密钥 {{name}}",
  },
  "Registered account {{username}}": {
    en: "Registered account {{username}}",
    zh: "注册账号 {{username}}",
  },
  "Operational Notifications": {
    en: "Operational Notifications",
    zh: "运营通知",
  },
  "Receive product announcements, campaigns and bonus notices by email. System emails such as verification codes and password resets are always sent.": {
    en: "Receive product announcements, campaigns and bonus notices by email. System emails such as verification codes and password resets are always sent.",
    zh: "通过邮件接收产品公告、运营活动与赠金到账等通知。验证码、找回密码等系统邮件不受影响，始终发送。",
  },
  "Filter users who turned off operational notifications": {
    en: "Filter users who turned off operational notifications",
    zh: "过滤已关闭运营通知的用户",
  },
  "Unchecking this sends to everyone, including users who turned off operational notifications.": {
    en: "Unchecking this sends to everyone, including users who turned off operational notifications.",
    zh: "取消勾选后会发送给已关闭运营通知的用户。",
  },
  "{{count}} users were skipped because they turned off operational notifications": {
    en: "{{count}} users were skipped because they turned off operational notifications",
    zh: "{{count}} 位用户已关闭运营通知，未发送邮件",
  },
  "Excluded Users": {
    en: "Excluded Users",
    zh: "排除用户",
  },
  "Search users to exclude": {
    en: "Search users to exclude",
    zh: "搜索要排除的用户",
  },
  "Excluded users are left out of the totals, charts and consumption details.": {
    en: "Excluded users are left out of the totals, charts and consumption details.",
    zh: "被排除的用户不会计入统计数字、图表和消费明细。",
  },
  "No users found": {
    en: "No users found",
    zh: "未找到用户",
  },
  "Searching...": {
    en: "Searching...",
    zh: "搜索中...",
  },
  "Remove {{username}}": {
    en: "Remove {{username}}",
    zh: "移除 {{username}}",
  },
  "Activity type": {
    en: "Activity type",
    zh: "活动类型",
  },
  "Add at least one benefit": {
    en: "Add at least one benefit",
    zh: "请至少添加一条权益",
  },
  "Add benefit": {
    en: "Add benefit",
    zh: "添加权益",
  },
  "An activity code grants a ratio discount or a list of free benefits to users for a limited time. Target groups must already exist in group ratios.": {
    en: "An activity code grants a ratio discount or a list of free benefits to users for a limited time. Target groups must already exist in group ratios.",
    zh: "活动码会在限定时间内给用户提供倍率优惠或一组免费权益。目标分组必须已在分组倍率中配置。",
  },
  "At most {{count}} benefits": {
    en: "At most {{count}} benefits",
    zh: "最多 {{count}} 条权益",
  },
  "At most {{count}} benefits can be added": {
    en: "At most {{count}} benefits can be added",
    zh: "最多只能添加 {{count}} 条权益",
  },
  "Benefits": {
    en: "Benefits",
    zh: "权益",
  },
  "Each benefit makes a whole group or a single model free. A token package caps the free usage, and usage beyond it falls back to normal pricing.": {
    en: "Each benefit makes a whole group or a single model free. A token package caps the free usage, and usage beyond it falls back to normal pricing.",
    zh: "每条权益可让整个分组或单个模型免费；Token 套餐会限制免费用量，超出后按原价计费。",
  },
  "Enter a valid token amount, for example 1000000, 1M or 100万": {
    en: "Enter a valid token amount, for example 1000000, 1M or 100万",
    zh: "请输入有效的 Token 数量，例如 1000000、1M 或 100万",
  },
  "For example: 1000000, 1M, 100万": {
    en: "For example: 1000000, 1M, 100万",
    zh: "例如：1000000、1M、100万",
  },
  "Free benefits": {
    en: "Free benefits",
    zh: "免费权益",
  },
  "Group for benefit {{index}}": {
    en: "Group for benefit {{index}}",
    zh: "第 {{index}} 条权益的分组",
  },
  "Model for benefit {{index}}": {
    en: "Model for benefit {{index}}",
    zh: "第 {{index}} 条权益的模型",
  },
  "No benefits yet.": {
    en: "No benefits yet.",
    zh: "暂无权益。",
  },
  "Normalized amount: {{amount}}": {
    en: "Normalized amount: {{amount}}",
    zh: "规范化数量：{{amount}}",
  },
  "Please enter a token amount": {
    en: "Please enter a token amount",
    zh: "请输入 Token 数量",
  },
  "Please select a model": {
    en: "Please select a model",
    zh: "请选择模型",
  },
  "Quota mode for benefit {{index}}": {
    en: "Quota mode for benefit {{index}}",
    zh: "第 {{index}} 条权益的额度模式",
  },
  "Ratio codes discount usage; free benefit codes grant free usage of chosen groups or models.": {
    en: "Ratio codes discount usage; free benefit codes grant free usage of chosen groups or models.",
    zh: "倍率活动码对用量打折；免费权益活动码让所选分组或模型免费使用。",
  },
  "Ratio discount": {
    en: "Ratio discount",
    zh: "倍率优惠",
  },
  "Redeem a code to unlock group access or a token package for a limited time.": {
    en: "Redeem a code to unlock group access or a token package for a limited time.",
    zh: "兑换活动码，限时解锁分组权限或 Token 套餐。",
  },
  "Remaining {{amount}}": {
    en: "Remaining {{amount}}",
    zh: "剩余 {{amount}}",
  },
  "Remove benefit {{index}}": {
    en: "Remove benefit {{index}}",
    zh: "移除第 {{index}} 条权益",
  },
  "RPM for benefit {{index}}": {
    en: "RPM for benefit {{index}}",
    zh: "第 {{index}} 条权益的 RPM",
  },
  "RPM limit must be a whole number": {
    en: "RPM limit must be a whole number",
    zh: "RPM 限制必须是整数",
  },
  "Saving immediately syncs active grants. Used tokens, expiry time and paid price stay unchanged; validity changes only affect new redemptions.": {
    en: "Saving immediately syncs active grants. Used tokens, expiry time and paid price stay unchanged; validity changes only affect new redemptions.",
    zh: "保存后会立即同步到有效授权。已用 Token、到期时间和已支付价格不变；有效期修改只影响之后的新兑换。",
  },
  "Scope for benefit {{index}}": {
    en: "Scope for benefit {{index}}",
    zh: "第 {{index}} 条权益的范围",
  },
  "Single model": {
    en: "Single model",
    zh: "单个模型",
  },
  "The activity type cannot be changed after creation.": {
    en: "The activity type cannot be changed after creation.",
    zh: "活动码类型创建后不可修改。",
  },
  "This benefit is already configured": {
    en: "This benefit is already configured",
    zh: "该权益已配置",
  },
  "Token amount for benefit {{index}}": {
    en: "Token amount for benefit {{index}}",
    zh: "第 {{index}} 条权益的 Token 数量",
  },
  "Token amount must be greater than 0": {
    en: "Token amount must be greater than 0",
    zh: "Token 数量必须大于 0",
  },
  "Token package": {
    en: "Token package",
    zh: "Token 套餐",
  },
  "Token quota used up (billed at the original price)": {
    en: "Token quota used up (billed at the original price)",
    zh: "Token 额度已用完（按原价计费）",
  },
  "Unlimited tokens": {
    en: "Unlimited tokens",
    zh: "不限量 Token",
  },
  "Used {{used}} / {{total}}": {
    en: "Used {{used}} / {{total}}",
    zh: "已用 {{used}} / {{total}}",
  },
  "Used {{used}} / {{total}} tokens": {
    en: "Used {{used}} / {{total}} tokens",
    zh: "已用 {{used}} / {{total}} Token",
  },
  "Used {{used}} tokens · Unlimited": {
    en: "Used {{used}} tokens · Unlimited",
    zh: "已用 {{used}} Token · 不限量",
  },
  "{{amount}} tokens": {
    en: "{{amount}} tokens",
    zh: "{{amount}} Token",
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

// 返回新键应按字母序插入的行号；键排在所有既有键之后时返回 -1，
// 由调用方改用「插到末键之前」的兼容分支。
function sortedInsertIndex(lines, key, from, to) {
  for (let i = from; i < to; i += 1) {
    const match = lines[i].match(/^\s*"((?:[^"\\]|\\.)*)"\s*:/)
    if (!match) continue
    if (JSON.parse('"' + match[1] + '"').localeCompare(key, 'en') > 0) return i
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
  if (translationStart < 0) {
    throw new Error('translation object not found in ' + file)
  }
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
      insertAt = sortedInsertIndex(lines, key, translationStart + 1, end)
    }
    if (insertAt < 0) {
      // Keep the trailing key unprefixed by inserting after the last key line.
      let lastKeyLine = -1
      for (let i = end - 1; i > translationStart; i -= 1) {
        if (/^\s*"(?:[^"\\]|\\.)*"\s*:/.test(lines[i])) {
          lastKeyLine = i
          break
        }
      }
      if (lastKeyLine < 0) {
        throw new Error('no key lines before the end of translation in ' + file)
      }
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