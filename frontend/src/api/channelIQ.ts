import { apiClient } from './client'

export type ChannelIQStatus = 'success' | 'failed' | 'pending'

export interface ChannelIQResult {
  id: number
  status: ChannelIQStatus | string
  model_id: string
  reasoning_effort: string
  response_text: string
  error_message?: string
  latency_ms: number
  started_at: string
  finished_at: string
  created_at: string
}

export interface ChannelIQPublicSnapshot {
  configured: boolean
  enabled: boolean
  model_id?: string
  reasoning_effort?: string
  results: ChannelIQResult[]
}

export async function get(): Promise<ChannelIQPublicSnapshot> {
  const { data } = await apiClient.get<ChannelIQPublicSnapshot>('/channel-iq')
  return data
}

export default { get }
