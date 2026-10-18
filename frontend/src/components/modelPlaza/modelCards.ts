import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { BILLING_MODE_IMAGE, BILLING_MODE_VIDEO } from '@/constants/channel'
import { resolveIntervalPrices } from '@/utils/pricing'

export interface PlazaModelVariant {
  group: ModelPlazaGroup
  model: PlazaModel
}

export interface PlazaModelEntry {
  key: string
  name: string
  platform: string
  variants: PlazaModelVariant[]
}

export function modelRate({ group, model }: PlazaModelVariant): number {
  if (model.pricing?.billing_mode === BILLING_MODE_IMAGE && group.image_rate_independent) {
    return group.image_rate_multiplier ?? 1
  }
  if (model.pricing?.billing_mode === BILLING_MODE_VIDEO && group.video_rate_independent) {
    return Math.max(0, group.video_rate_multiplier ?? 1)
  }
  return group.user_rate_multiplier ?? group.rate_multiplier
}

/** Match the detailed table's lowest context tier, including relative prices. */
export function baseTokenPrices(model: PlazaModel) {
  const pricing = model.pricing
  if (!pricing) return null
  const first = [...pricing.intervals].sort((a, b) => a.min_tokens - b.min_tokens)[0]
  return first ? resolveIntervalPrices(first, pricing) : pricing
}
