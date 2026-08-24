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
import { describe, expect, it } from 'vitest'

import { isValidVipTierLadder } from '../vip-tiers'

const ladder = (tiers: unknown) => JSON.stringify(tiers)

describe('isValidVipTierLadder', () => {
  it('accepts a ladder whose thresholds increase', () => {
    expect(
      isValidVipTierLadder(
        ladder([
          { key: 'vip1', group: 'vip1', min_spend: 2_500_000, enabled: true },
          { key: 'vip2', group: 'vip2', min_spend: 7_500_000, enabled: true },
        ])
      )
    ).toBe(true)
  })

  it('accepts an empty value so the field can be left untouched', () => {
    expect(isValidVipTierLadder('')).toBe(true)
    expect(isValidVipTierLadder(undefined)).toBe(true)
  })

  it('rejects thresholds that do not increase', () => {
    expect(
      isValidVipTierLadder(
        ladder([
          { key: 'vip1', group: 'vip1', min_spend: 7_500_000 },
          { key: 'vip2', group: 'vip2', min_spend: 2_500_000 },
        ])
      )
    ).toBe(false)
  })

  it('rejects a duplicated key or group', () => {
    expect(
      isValidVipTierLadder(
        ladder([
          { key: 'vip1', group: 'vip1', min_spend: 1_000 },
          { key: 'vip1', group: 'vip2', min_spend: 2_000 },
        ])
      )
    ).toBe(false)
    expect(
      isValidVipTierLadder(
        ladder([
          { key: 'vip1', group: 'vip1', min_spend: 1_000 },
          { key: 'vip2', group: 'vip1', min_spend: 2_000 },
        ])
      )
    ).toBe(false)
  })

  it('rejects a missing group, a non-positive threshold and malformed JSON', () => {
    expect(
      isValidVipTierLadder(ladder([{ key: 'vip1', min_spend: 1_000 }]))
    ).toBe(false)
    expect(
      isValidVipTierLadder(
        ladder([{ key: 'vip1', group: 'vip1', min_spend: 0 }])
      )
    ).toBe(false)
    expect(isValidVipTierLadder('[{')).toBe(false)
    expect(isValidVipTierLadder('{"vip1": 1}')).toBe(false)
  })
})
