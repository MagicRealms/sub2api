import { apiClient } from '../client'

export interface ChannelIQConfig {
  id: number
  account_id: number
  account_name: string
  account_platform: string
  model_id: string
  prompt: string
  reasoning_effort: string
  interval_seconds: number
  enabled: boolean
  last_run_at?: string
  next_run_at?: string
  updated_at?: string
}

export interface ChannelIQResult {
  id: number
  status: string
  model_id: string
  reasoning_effort: string
  response_text: string
  error_message?: string
  latency_ms: number
  started_at: string
  finished_at: string
  created_at: string
}

export interface ChannelIQSnapshot {
  config: ChannelIQConfig | null
  results: ChannelIQResult[]
}

export interface SaveChannelIQConfigRequest {
  account_id: number
  model_id: string
  prompt: string
  reasoning_effort: string
  interval_seconds: number
  enabled: boolean
}

export async function get(): Promise<ChannelIQSnapshot> {
  const { data } = await apiClient.get<ChannelIQSnapshot>('/admin/channel-iq')
  return data
}

export async function save(input: SaveChannelIQConfigRequest): Promise<ChannelIQSnapshot> {
  const { data } = await apiClient.put<ChannelIQSnapshot>('/admin/channel-iq', input)
  return data
}

export async function run(): Promise<ChannelIQResult> {
  const { data } = await apiClient.post<ChannelIQResult>('/admin/channel-iq/run')
  return data
}

export default { get, save, run }
