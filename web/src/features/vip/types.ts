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
export type ApiResponse<T> = {
  success: boolean
  message: string
  data: T
}

export type VipTier = {
  key: string
  group: string
  /** Qualifying spend threshold, in raw quota units. */
  min_spend: number
  enabled: boolean
}

export type VipLadder = {
  enabled: boolean
  auto_promote_enabled: boolean
  window_days: number
  tiers: VipTier[]
  quota_per_unit: number
}

export type VipStatus = {
  enabled: boolean
  tier: string
  group: string
  expires_at: number
  locked: boolean
  spend: number
  total_paid: number
  next_tier: string
  next_tier_min_spend: number
  window_days: number
  /** A paid subscription owns the group, so the tier is recorded but not applied. */
  subscription_held: boolean
}

export type SetVipTierRequest = {
  tier: string
  days: number
  locked: boolean
}
