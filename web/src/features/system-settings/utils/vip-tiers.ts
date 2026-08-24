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

/**
 * Shape check for the VIP ladder before it is sent to the server, which
 * revalidates it and additionally enforces that group ratios fall as
 * thresholds rise. Duplicated keys, duplicated groups and thresholds that do
 * not increase would silently produce a ladder where a cheaper level prices
 * below a dearer one, so they are refused here too.
 */
export function isValidVipTierLadder(value: string | undefined): boolean {
  if (!value || value.trim() === '') return true
  let parsed: unknown
  try {
    parsed = JSON.parse(value)
  } catch {
    return false
  }
  if (!Array.isArray(parsed)) return false

  const keys = new Set<string>()
  const groups = new Set<string>()
  let previousMinSpend = 0

  for (const entry of parsed) {
    if (typeof entry !== 'object' || entry === null) return false
    const tier = entry as Record<string, unknown>
    if (typeof tier.key !== 'string' || tier.key.trim() === '') return false
    if (typeof tier.group !== 'string' || tier.group.trim() === '') return false
    if (
      typeof tier.min_spend !== 'number' ||
      !Number.isFinite(tier.min_spend)
    ) {
      return false
    }
    if (tier.min_spend <= 0 || tier.min_spend > Number.MAX_SAFE_INTEGER) {
      return false
    }
    if ('enabled' in tier && typeof tier.enabled !== 'boolean') return false
    if (keys.has(tier.key) || groups.has(tier.group)) return false
    keys.add(tier.key)
    groups.add(tier.group)
    if (tier.min_spend <= previousMinSpend) return false
    previousMinSpend = tier.min_spend
  }

  return true
}
