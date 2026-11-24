export interface AccountFinalFallbackConfig {
  enabled: boolean
  groupIds: number[]
}

export function readAccountFinalFallback(extra?: Record<string, unknown> | null): AccountFinalFallbackConfig {
  const values = extra?.account_final_fallback_group_ids
  return {
    enabled: extra?.account_final_fallback_enabled === true,
    groupIds: Array.isArray(values)
      ? [...new Set(values.filter((value): value is number => Number.isSafeInteger(value) && value > 0))]
      : []
  }
}

export function writeAccountFinalFallback(
  extra: Record<string, unknown> | undefined,
  config: AccountFinalFallbackConfig
): Record<string, unknown> {
  return {
    ...extra,
    account_final_fallback_enabled: config.enabled,
    account_final_fallback_group_ids: config.enabled ? [...config.groupIds] : []
  }
}

export function normalAccountGroupIds(groupIds: number[], config: AccountFinalFallbackConfig): number[] {
  return config.enabled ? groupIds.filter(id => !config.groupIds.includes(id)) : [...groupIds]
}
