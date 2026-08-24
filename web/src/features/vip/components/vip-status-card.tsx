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
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import {
  Progress,
  ProgressIndicator,
  ProgressTrack,
} from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuotaWithCurrency } from '@/lib/currency'
import dayjs from '@/lib/dayjs'

import { useVipSelf } from '../hooks/use-vip'
import { computeVipProgress } from '../lib/progress'

export function VipStatusCard() {
  const { t } = useTranslation()
  const vipSelf = useVipSelf()

  if (vipSelf.isLoading) {
    return <Skeleton className='h-32 w-full' />
  }

  const status = vipSelf.data
  if (!status || !status.enabled) {
    return null
  }

  const progress = computeVipProgress(status.spend, status.next_tier_min_spend)

  return (
    <Card data-card-hover='false' className='gap-3 p-4'>
      <div className='flex items-center justify-between gap-2'>
        <h3 className='text-sm font-medium'>{t('VIP Level')}</h3>
        {status.tier ? (
          <Badge variant='default'>{status.tier.toUpperCase()}</Badge>
        ) : (
          <Badge variant='outline'>{t('No VIP level yet')}</Badge>
        )}
      </div>

      <dl className='text-muted-foreground grid grid-cols-2 gap-2 text-xs'>
        <div>
          <dt>{t('Spend in current window')}</dt>
          <dd className='text-foreground'>
            {formatQuotaWithCurrency(status.spend)}
          </dd>
        </div>
        <div>
          <dt>{t('Window length')}</dt>
          <dd className='text-foreground'>
            {t('{{count}} days', { count: status.window_days })}
          </dd>
        </div>
        {status.expires_at > 0 && (
          <div>
            <dt>{t('Valid until')}</dt>
            <dd className='text-foreground'>
              {dayjs.unix(status.expires_at).format('YYYY-MM-DD HH:mm')}
            </dd>
          </div>
        )}
        <div>
          <dt>{t('Lifetime paid')}</dt>
          <dd className='text-foreground'>
            {formatQuotaWithCurrency(status.total_paid)}
          </dd>
        </div>
      </dl>

      {status.next_tier && (
        <div className='space-y-1'>
          <Progress value={progress.percent}>
            <ProgressTrack>
              <ProgressIndicator />
            </ProgressTrack>
          </Progress>
          <p className='text-muted-foreground text-xs'>
            {t('{{amount}} more to reach {{tier}}', {
              amount: formatQuotaWithCurrency(progress.remaining),
              tier: status.next_tier.toUpperCase(),
            })}
          </p>
        </div>
      )}

      {status.locked && (
        <p className='text-muted-foreground text-xs'>
          {t(
            'This level was set by an administrator and does not expire on its own.'
          )}
        </p>
      )}

      {status.subscription_held && (
        <p className='text-muted-foreground text-xs'>
          {t(
            'An active subscription currently controls your group, so the VIP group is not applied.'
          )}
        </p>
      )}
    </Card>
  )
}
