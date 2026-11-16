import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { BILLING_MODE_IMAGE, BILLING_MODE_VIDEO } from '@/constants/channel'
import { resolveIntervalPrices, type PricingCurrency } from '@/utils/pricing'
import { platformLabel } from '@/utils/platformColors'

export function modelPlazaPlatformLabel(platform: string): string {
  return platform === 'sharellm' ? '聚合站' : platformLabel(platform)
}

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

/** Cards and detail tables use the currency declared by the same price card. */
export function modelPricingCurrency(model: PlazaModel): PricingCurrency {
  return model.official_pricing?.currency === 'CNY' ? 'CNY' : 'USD'
}

/** Model capability is independent of billing: image models may bill by token. */
export function modelKind(model: Pick<PlazaModel, 'name' | 'pricing'>): 'chat' | 'image' | 'audio' | 'video' {
  if (model.pricing?.billing_mode === BILLING_MODE_VIDEO || /(?:^|[-_/])(video|sora|veo|kling|wan)(?:[-_/.]|$)/i.test(model.name)) return 'video'
  if (model.pricing?.billing_mode === BILLING_MODE_IMAGE || /image|dall-e|imagine|imagen|flux/i.test(model.name)) return 'image'
  if (/audio|realtime|whisper|(?:^|[-_/])(tts|stt)(?:[-_/.]|$)/i.test(model.name)) return 'audio'
  return 'chat'
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
