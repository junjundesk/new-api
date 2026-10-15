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
  "Activity Codes": {
    en: "Activity Codes",
    zh: "活动码",
  },
  "Activity Code": {
    en: "Activity Code",
    zh: "活动码",
  },
  "Create Activity Code": {
    en: "Create Activity Code",
    zh: "创建活动码",
  },
  "Update Activity Code": {
    en: "Update Activity Code",
    zh: "编辑活动码",
  },
  "An activity code grants its target group to the user for a limited time. The group must already exist in group ratios.": {
    en: "An activity code grants its target group to the user for a limited time. The group must already exist in group ratios.",
    zh: "活动码在有效期内把目标分组授权给用户；目标分组必须已在分组倍率中配置。",
  },
  "For example: hdmfree": {
    en: "For example: hdmfree",
    zh: "例如：hdmfree",
  },
  "Lowercase letters, digits, _ and - only. The code cannot be changed after creation.": {
    en: "Lowercase letters, digits, _ and - only. The code cannot be changed after creation.",
    zh: "只能使用小写字母、数字、_ 和 -；活动码创建后不可修改。",
  },
  "Enter a name": {
    en: "Enter a name",
    zh: "请输入名称",
  },
  "Optional description": {
    en: "Optional description",
    zh: "选填描述",
  },
  "Target Group": {
    en: "Target Group",
    zh: "目标分组",
  },
  "Select a group": {
    en: "Select a group",
    zh: "请选择分组",
  },
  "Users with an active grant can switch their token group to it.": {
    en: "Users with an active grant can switch their token group to it.",
    zh: "用户获得授权后，把令牌分组切换到该分组即可使用。",
  },
  "Pricing and Limits": {
    en: "Pricing and Limits",
    zh: "价格与限制",
  },
  "Validity (hours)": {
    en: "Validity (hours)",
    zh: "有效期（小时）",
  },
  "0 means the grant never expires.": {
    en: "0 means the grant never expires.",
    zh: "0 表示授权永久有效。",
  },
  "RPM limit": {
    en: "RPM limit",
    zh: "RPM 限制",
  },
  "Requests per minute. 0 means unlimited.": {
    en: "Requests per minute. 0 means unlimited.",
    zh: "每分钟请求数上限，0 表示不限。",
  },
  "Price ({{currency}})": {
    en: "Price ({{currency}})",
    zh: "价格（{{currency}}）",
  },
  "Paid with wallet balance only. 0 means the code is free.": {
    en: "Paid with wallet balance only. 0 means the code is free.",
    zh: "仅能用真余额支付；0 表示免费。",
  },
  "Group Default Ratio": {
    en: "Group Default Ratio",
    zh: "分组默认倍率",
  },
  "Custom Ratio": {
    en: "Custom Ratio",
    zh: "自定义倍率",
  },
  "Custom ratio": {
    en: "Custom ratio",
    zh: "自定义倍率",
  },
  "0 means this group is completely free.": {
    en: "0 means this group is completely free.",
    zh: "0 表示该分组完全免费。",
  },
  "Total redemptions": {
    en: "Total redemptions",
    zh: "总可兑换次数",
  },
  "Redemptions per user": {
    en: "Redemptions per user",
    zh: "每人可兑换次数",
  },
  "0 means unlimited.": {
    en: "0 means unlimited.",
    zh: "0 表示不限。",
  },
  "0 means unlimited. Extra redemptions extend the expiry.": {
    en: "0 means unlimited. Extra redemptions extend the expiry.",
    zh: "0 表示不限；重复兑换会顺延到期时间。",
  },
  "Redemption starts": {
    en: "Redemption starts",
    zh: "可兑换开始时间",
  },
  "Redemption ends": {
    en: "Redemption ends",
    zh: "可兑换结束时间",
  },
  "Show in activity center": {
    en: "Show in activity center",
    zh: "在活动中心展示",
  },
  "Users can redeem it directly from their wallet.": {
    en: "Users can redeem it directly from their wallet.",
    zh: "用户可直接在钱包中兑换。",
  },
  "Delete Activity Code?": {
    en: "Delete Activity Code?",
    zh: "删除活动码？",
  },
  "This will delete the activity code": {
    en: "This will delete the activity code",
    zh: "将删除活动码",
  },
  ". Grants that were already redeemed stay active until they expire.": {
    en: ". Grants that were already redeemed stay active until they expire.",
    zh: "；已兑换的授权会保留到自然到期。",
  },
  "Activity Grants": {
    en: "Activity Grants",
    zh: "活动授权",
  },
  "Redeemed grants for this activity code. Revoking one takes effect immediately.": {
    en: "Redeemed grants for this activity code. Revoking one takes effect immediately.",
    zh: "该活动码的兑换记录，撤销后立即生效。",
  },
  "No grants yet.": {
    en: "No grants yet.",
    zh: "暂无兑换记录。",
  },
  "Group": {
    en: "Group",
    zh: "分组",
  },
  "Validity": {
    en: "Validity",
    zh: "有效期",
  },
  "Free": {
    en: "Free",
    zh: "免费",
  },
  "Inherited": {
    en: "Inherited",
    zh: "继承分组倍率",
  },
  "Fully Free": {
    en: "Fully Free",
    zh: "完全免费",
  },
  "RPM": {
    en: "RPM",
    zh: "RPM",
  },
  "Unlimited": {
    en: "Unlimited",
    zh: "不限",
  },
  "Limits": {
    en: "Limits",
    zh: "限额",
  },
  "Total": {
    en: "Total",
    zh: "总量",
  },
  "Per user": {
    en: "Per user",
    zh: "每人",
  },
  "Redemption Window": {
    en: "Redemption Window",
    zh: "可兑换时间",
  },
  "Always": {
    en: "Always",
    zh: "长期开放",
  },
  "Until": {
    en: "Until",
    zh: "截止",
  },
  "From": {
    en: "From",
    zh: "开始于",
  },
  "Ended": {
    en: "Ended",
    zh: "已结束",
  },
  "No Activity Codes Found": {
    en: "No Activity Codes Found",
    zh: "暂无活动码",
  },
  "No activity codes yet. Create one to grant a temporary group to your users.": {
    en: "No activity codes yet. Create one to grant a temporary group to your users.",
    zh: "还没有活动码。创建一个即可向用户发放限时分组。",
  },
  "Filter by code or name...": {
    en: "Filter by code or name...",
    zh: "按活动码或名称筛选...",
  },
  "Grants": {
    en: "Grants",
    zh: "授权记录",
  },
  "Expires at": {
    en: "Expires at",
    zh: "到期时间",
  },
  "Paid": {
    en: "Paid",
    zh: "已支付",
  },
  "Revoke": {
    en: "Revoke",
    zh: "撤销",
  },
  "Failed to load activity codes": {
    en: "Failed to load activity codes",
    zh: "加载活动码失败",
  },
  "Failed to search activity codes": {
    en: "Failed to search activity codes",
    zh: "搜索活动码失败",
  },
  "Failed to create activity code": {
    en: "Failed to create activity code",
    zh: "创建活动码失败",
  },
  "Failed to update activity code": {
    en: "Failed to update activity code",
    zh: "更新活动码失败",
  },
  "Failed to delete activity code": {
    en: "Failed to delete activity code",
    zh: "删除活动码失败",
  },
  "Failed to update activity code status": {
    en: "Failed to update activity code status",
    zh: "更新活动码状态失败",
  },
  "Failed to load activity grants": {
    en: "Failed to load activity grants",
    zh: "加载活动授权失败",
  },
  "Failed to revoke activity grant": {
    en: "Failed to revoke activity grant",
    zh: "撤销活动授权失败",
  },
  "Activity code created": {
    en: "Activity code created",
    zh: "活动码已创建",
  },
  "Activity code updated": {
    en: "Activity code updated",
    zh: "活动码已更新",
  },
  "Activity code deleted": {
    en: "Activity code deleted",
    zh: "活动码已删除",
  },
  "Activity code disabled": {
    en: "Activity code disabled",
    zh: "活动码已停用",
  },
  "Activity code enabled": {
    en: "Activity code enabled",
    zh: "活动码已启用",
  },
  "Activity grant revoked": {
    en: "Activity grant revoked",
    zh: "活动授权已撤销",
  },
  "Please enter an activity code": {
    en: "Please enter an activity code",
    zh: "请输入活动码",
  },
  "Activity code must be 1-64 characters of a-z, 0-9, _ or -": {
    en: "Activity code must be 1-64 characters of a-z, 0-9, _ or -",
    zh: "活动码只能是 1-64 位的 a-z、0-9、_ 或 -",
  },
  "Please enter a name": {
    en: "Please enter a name",
    zh: "请输入名称",
  },
  "Name must be 1-64 characters": {
    en: "Name must be 1-64 characters",
    zh: "名称长度必须在 1-64 之间",
  },
  "Please select a group": {
    en: "Please select a group",
    zh: "请选择分组",
  },
  "Validity must be between 0 and 87600 hours": {
    en: "Validity must be between 0 and 87600 hours",
    zh: "有效期必须在 0 到 87600 小时之间",
  },
  "RPM limit must be 0 or greater": {
    en: "RPM limit must be 0 or greater",
    zh: "RPM 限制不能为负数",
  },
  "RPM limit is too large": {
    en: "RPM limit is too large",
    zh: "RPM 限制过大",
  },
  "Price must be zero or greater": {
    en: "Price must be zero or greater",
    zh: "价格不能为负数",
  },
  "Ratio must be between 0 and 1000": {
    en: "Ratio must be between 0 and 1000",
    zh: "倍率必须在 0 到 1000 之间",
  },
  "Total limit must be 0 or greater": {
    en: "Total limit must be 0 or greater",
    zh: "总可兑换次数不能为负数",
  },
  "Total limit is too large": {
    en: "Total limit is too large",
    zh: "总可兑换次数过大",
  },
  "Per-user limit must be 0 or greater": {
    en: "Per-user limit must be 0 or greater",
    zh: "每人可兑换次数不能为负数",
  },
  "Per-user limit is too large": {
    en: "Per-user limit is too large",
    zh: "每人可兑换次数过大",
  },
  "{{count}} minutes": {
    en: "{{count}} minutes",
    zh: "{{count}} 分钟",
  },
  "{{count}} hours": {
    en: "{{count}} hours",
    zh: "{{count}} 小时",
  },
  "{{count}} days": {
    en: "{{count}} days",
    zh: "{{count}} 天",
  },
  "Permanent": {
    en: "Permanent",
    zh: "永久",
  },
  "Redeem a code to unlock a group for a limited time and at a special ratio.": {
    en: "Redeem a code to unlock a group for a limited time and at a special ratio.",
    zh: "输入活动码，即可在限定时间内以专属倍率使用指定分组。",
  },
  "Enter your activity code": {
    en: "Enter your activity code",
    zh: "请输入活动码",
  },
  "Verify": {
    en: "Verify",
    zh: "校验",
  },
  "Free activity codes are redeemed right away; paid ones ask for confirmation first.": {
    en: "Free activity codes are redeemed right away; paid ones ask for confirmation first.",
    zh: "免费活动码直接兑换成功，付费活动码会先弹出确认。",
  },
  "Available Activities": {
    en: "Available Activities",
    zh: "可参与的活动",
  },
  "Confirm Redemption": {
    en: "Confirm Redemption",
    zh: "确认兑换",
  },
  "Spend {{quota}} of your wallet balance to redeem {{group}}?": {
    en: "Spend {{quota}} of your wallet balance to redeem {{group}}?",
    zh: "是否消耗 {{quota}} 余额兑换 {{group}}？",
  },
  "Wallet balance": {
    en: "Wallet balance",
    zh: "钱包余额",
  },
  "Bonus credit cannot be used for redemption.": {
    en: "Bonus credit cannot be used for redemption.",
    zh: "赠金不能用于兑换。",
  },
  "Confirm and Redeem": {
    en: "Confirm and Redeem",
    zh: "确认兑换",
  },
  "Redeemed {{group}}. Switch your token group to start using it.": {
    en: "Redeemed {{group}}. Switch your token group to start using it.",
    zh: "已兑换 {{group}}。把令牌分组切换到该分组即可使用。",
  },
  "Invalid activity code": {
    en: "Invalid activity code",
    zh: "无效的活动码",
  },
  "This activity code is not redeemable now": {
    en: "This activity code is not redeemable now",
    zh: "该活动码当前不可兑换",
  },
  "Failed to load activity offers": {
    en: "Failed to load activity offers",
    zh: "加载活动列表失败",
  },
  "Group default ratio": {
    en: "Group default ratio",
    zh: "分组默认倍率",
  },
  "Fully free": {
    en: "Fully free",
    zh: "完全免费",
  },
  "My Activity Grants": {
    en: "My Activity Grants",
    zh: "我的活动授权",
  },
  "Set a token group to the granted group to use it while the grant is active.": {
    en: "Set a token group to the granted group to use it while the grant is active.",
    zh: "把令牌分组切换到已授权的分组即可在有效期内使用。",
  },
  "No activity grants yet.": {
    en: "No activity grants yet.",
    zh: "暂无活动授权。",
  },
  "Activity Center": {
    en: "Activity Center",
    zh: "活动中心",
  },
  "Redeem an Activity Code": {
    en: "Redeem an Activity Code",
    zh: "兑换活动码",
  },
  "Free codes are redeemed instantly. Paid codes ask you to confirm the wallet deduction first.": {
    en: "Free codes are redeemed instantly. Paid codes ask you to confirm the wallet deduction first.",
    zh: "免费活动码即时兑换；付费活动码需要先确认余额扣减。",
  },
  "Active": {
    en: "Active",
    zh: "有效",
  },
  "Revoked": {
    en: "Revoked",
    zh: "已撤销",
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