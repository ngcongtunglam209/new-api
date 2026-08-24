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
export type VipProgress = {
  /** Completed share of the way to the next tier, 0-100. */
  percent: number
  /** Quota units still needed, 0 once the threshold is cleared. */
  remaining: number
}

/**
 * Progress from the spend accumulated in the current window toward the next
 * tier. A missing or non-positive threshold means there is no tier left to
 * climb toward, which reads as complete rather than as an error.
 */
export function computeVipProgress(
  spend: number,
  nextMinSpend: number
): VipProgress {
  const safeSpend = Number.isFinite(spend) && spend > 0 ? spend : 0
  if (!Number.isFinite(nextMinSpend) || nextMinSpend <= 0) {
    return { percent: 100, remaining: 0 }
  }
  const remaining = Math.max(0, nextMinSpend - safeSpend)
  const percent = Math.min(100, Math.floor((safeSpend / nextMinSpend) * 100))
  return { percent, remaining }
}
