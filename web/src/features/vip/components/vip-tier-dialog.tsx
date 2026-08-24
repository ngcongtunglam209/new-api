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
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { cn } from '@/lib/utils'

import { clearUserVipTier, setUserVipTier } from '../api'
import { useUserVipStatus, useVipLadder, vipQueryKeys } from '../hooks/use-vip'

interface VipTierDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  username: string
  onSuccess?: () => void
}

export function VipTierDialog(props: VipTierDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const ladder = useVipLadder()
  const status = useUserVipStatus(props.userId, props.open)
  const [tier, setTier] = useState('')
  const [days, setDays] = useState('')
  const [locked, setLocked] = useState(false)
  const [loading, setLoading] = useState(false)

  const selectedTier = tier || status.data?.tier || ''
  const tiers = ladder.data?.tiers ?? []

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: vipQueryKeys.user(props.userId) })
    props.onSuccess?.()
  }

  const handleSave = async () => {
    if (!selectedTier) {
      toast.error(t('Select a VIP level first'))
      return
    }
    setLoading(true)
    try {
      const result = await setUserVipTier(props.userId, {
        tier: selectedTier,
        days: Number.parseInt(days, 10) || 0,
        locked,
      })
      if (!result.success) {
        toast.error(result.message || t('Failed to update VIP level'))
        return
      }
      toast.success(t('VIP level updated'))
      refresh()
      props.onOpenChange(false)
    } catch (e: unknown) {
      toast.error(
        e instanceof Error ? e.message : t('Failed to update VIP level')
      )
    } finally {
      setLoading(false)
    }
  }

  const handleClear = async () => {
    setLoading(true)
    try {
      const result = await clearUserVipTier(props.userId)
      if (!result.success) {
        toast.error(result.message || t('Failed to update VIP level'))
        return
      }
      toast.success(t('VIP level removed'))
      refresh()
      props.onOpenChange(false)
    } catch (e: unknown) {
      toast.error(
        e instanceof Error ? e.message : t('Failed to update VIP level')
      )
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('VIP Level')}
      description={t('Set or remove the VIP level of {{username}}', {
        username: props.username,
      })}
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            variant='outline'
            onClick={handleClear}
            disabled={loading || !status.data?.tier}
          >
            {t('Remove level')}
          </Button>
          <Button onClick={handleSave} disabled={loading}>
            {loading ? t('Processing...') : t('Confirm')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        {!ladder.data?.enabled && (
          <p className='text-destructive text-sm'>
            {t(
              'The VIP system is disabled, enable it in billing settings first.'
            )}
          </p>
        )}

        <div className='text-muted-foreground text-sm'>
          {t('Current level')}:{' '}
          <span className='text-foreground'>
            {status.data?.tier ? status.data.tier.toUpperCase() : t('None')}
          </span>
          {' · '}
          {t('Group')}:{' '}
          <span className='text-foreground'>{status.data?.group ?? '-'}</span>
        </div>

        <div className='space-y-2'>
          <Label>{t('VIP Level')}</Label>
          <div className='flex flex-wrap gap-1'>
            {tiers.map((item) => (
              <Button
                key={item.key}
                type='button'
                variant='outline'
                size='sm'
                aria-pressed={selectedTier === item.key}
                className={cn(
                  selectedTier === item.key &&
                    'bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground'
                )}
                onClick={() => setTier(item.key)}
              >
                {item.key.toUpperCase()} ·{' '}
                {formatQuotaWithCurrency(item.min_spend)}
              </Button>
            ))}
          </div>
        </div>

        <div className='space-y-2'>
          <Label htmlFor='vip-days'>{t('Valid for (days)')}</Label>
          <Input
            id='vip-days'
            type='number'
            min={0}
            placeholder={String(ladder.data?.window_days ?? 90)}
            value={days}
            onChange={(e) => setDays(e.target.value)}
          />
          <p className='text-muted-foreground text-xs'>
            {t('Leave empty to use the configured window length.')}
          </p>
        </div>

        <div className='flex items-center justify-between gap-2'>
          <Label htmlFor='vip-locked'>{t('Pin this level')}</Label>
          <Switch
            id='vip-locked'
            checked={locked}
            onCheckedChange={(value: boolean) => setLocked(value)}
          />
        </div>
        <p className='text-muted-foreground text-xs'>
          {t('A pinned level never expires and is never changed by spend.')}
        </p>
      </div>
    </Dialog>
  )
}
