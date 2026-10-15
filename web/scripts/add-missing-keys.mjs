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