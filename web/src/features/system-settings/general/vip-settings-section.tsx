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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { JsonCodeEditor } from '@/components/json-code-editor'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { isValidVipTierLadder } from '../utils/vip-tiers'

const createVipSchema = (t: (key: string) => string) =>
  z.object({
    'vip_setting.enabled': z.boolean(),
    'vip_setting.auto_promote_enabled': z.boolean(),
    'vip_setting.window_days': z.number().min(1).max(3650),
    'vip_setting.redemption_exclude_prefixes': z.string(),
    'vip_setting.tiers': z.string().refine(isValidVipTierLadder, {
      message: t(
        'Each tier needs a key, a group and a min_spend, and thresholds must increase'
      ),
    }),
  })

type VipFormValues = z.infer<ReturnType<typeof createVipSchema>>

type VipSettingsSectionProps = {
  defaultValues: VipFormValues
}

export function VipSettingsSection(props: VipSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<VipFormValues>({
    resolver: zodResolver(createVipSchema(t)),
    mode: 'onChange',
    defaultValues: props.defaultValues,
  })

  useEffect(() => {
    form.reset(props.defaultValues)
  }, [props.defaultValues, form])

  const onSubmit = async (values: VipFormValues) => {
    const updates = Object.entries(values).filter(
      ([key, value]) =>
        value !== props.defaultValues[key as keyof VipFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: value ?? '' })
    }
  }

  return (
    <SettingsSection title={t('VIP Levels')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <FormField
            control={form.control}
            name='vip_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable VIP levels')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Grant a group by VIP level. Configure the group ratio and rate limit of each VIP group first, otherwise the level grants nothing.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='vip_setting.auto_promote_enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Promote automatically by spend')}</FormLabel>
                  <FormDescription>
                    {t(
                      'While off, spend is still recorded but levels only change when an administrator sets them.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='vip_setting.window_days'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Window length (days)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    value={field.value}
                    onChange={(e) => field.onChange(Number(e.target.value))}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'A level lasts this long, every payment extends it, and spend restarts from zero once it lapses.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='vip_setting.redemption_exclude_prefixes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('Redemption code prefixes that are not sales')}
                </FormLabel>
                <FormControl>
                  <Input placeholder='gift-,test-' {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Comma separated. Codes whose name starts with one of these do not count as money paid.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='vip_setting.tiers'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Level ladder')}</FormLabel>
                <FormControl>
                  <JsonCodeEditor
                    value={field.value || ''}
                    onChange={field.onChange}
                    name={field.name}
                    onBlur={field.onBlur}
                    textareaRef={field.ref}
                    aria-invalid={Boolean(
                      form.formState.errors['vip_setting.tiers']
                    )}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'min_spend is in raw quota units, so a $5 threshold with QuotaPerUnit 500000 is 2500000.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save VIP levels'
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
