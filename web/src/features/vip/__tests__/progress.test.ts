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

import { computeVipProgress } from '../lib/progress'

describe('computeVipProgress', () => {
  it('reports the remaining spend and share when a next tier exists', () => {
    expect(computeVipProgress(2_500_000, 7_500_000)).toEqual({
      percent: 33,
      remaining: 5_000_000,
    })
  })

  it('reports complete when the highest tier leaves no next threshold', () => {
    expect(computeVipProgress(50_000_000, 0)).toEqual({
      percent: 100,
      remaining: 0,
    })
  })

  it('caps at complete once the threshold is cleared', () => {
    expect(computeVipProgress(9_000_000, 7_500_000)).toEqual({
      percent: 100,
      remaining: 0,
    })
  })

  it('treats missing spend as no progress instead of failing', () => {
    expect(computeVipProgress(Number.NaN, 7_500_000)).toEqual({
      percent: 0,
      remaining: 7_500_000,
    })
  })
})
