import { apiClient } from './client'

export interface TesterMetric {
  model: string
  group_id: number
  group_name: string
  requests: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_creation_tokens: number
  total_cost: number
  actual_cost: number
  cache_hit_rate: number
  first_token_samples: number
  output_samples: number
  first_token_p50_ms: number | null
  first_token_p95_ms: number | null
  duration_p50_ms: number | null
  duration_p95_ms: number | null
  output_tokens_per_second: number | null
  cost_per_million_tokens: number | null
}

export interface TesterSnapshot {
  generated_at: string
  start_time: string
  end_time: string
  visible_after: string | null
  summary: TesterMetric
  models: TesterMetric[]
  models_truncated: boolean
  error_records: number
}

export interface TesterParams {
  start_time: string
  end_time: string
  timezone: string
  api_key_id?: number
  group_id?: number
  model?: string
}

export async function getTesterSnapshot(params: TesterParams, signal?: AbortSignal): Promise<TesterSnapshot> {
  const { data } = await apiClient.get<TesterSnapshot>('/tester/monitor', { params, signal })
  return data
}

export async function resetTesterHistory(): Promise<{ visible_after: string; billing_preserved: boolean }> {
  const { data } = await apiClient.post('/tester/usage/reset')
  return data
}
