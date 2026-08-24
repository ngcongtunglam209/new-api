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
import { useQuery } from '@tanstack/react-query'

import { getUserVipStatus, getVipLadder, getVipSelf } from '../api'

export const vipQueryKeys = {
  ladder: ['vip', 'ladder'] as const,
  self: ['vip', 'self'] as const,
  user: (userId: number) => ['vip', 'user', userId] as const,
}

export function useVipLadder() {
  return useQuery({
    queryKey: vipQueryKeys.ladder,
    queryFn: async () => (await getVipLadder()).data,
  })
}

export function useVipSelf() {
  return useQuery({
    queryKey: vipQueryKeys.self,
    queryFn: async () => (await getVipSelf()).data,
  })
}

export function useUserVipStatus(userId: number, enabled: boolean) {
  return useQuery({
    queryKey: vipQueryKeys.user(userId),
    queryFn: async () => (await getUserVipStatus(userId)).data,
    enabled: enabled && userId > 0,
  })
}
