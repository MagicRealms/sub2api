import { apiClient } from './client'

export type ChannelIQStatus = 'disabled' | 'unconfigured' | 'pending' | 'healthy' | 'warning' | 'error'

export interface ChannelIQItem {
  channel_id: number
  channel_name: string
  group_id?: number
  group_name?: string
  platform?: string
  model?: string
  status: ChannelIQStatus
  latency_ms?: number
  response_preview?: string
  error?: string
  checked_at?: string
  next_check_at?: string
  interval_seconds?: number
}

export async function list(): Promise<{ items: ChannelIQItem[] }> {
  const { data } = await apiClient.get<{ items: ChannelIQItem[] }>('/channel-iq')
  return data
}

export async function run(channelId: number): Promise<ChannelIQItem> {
  const { data } = await apiClient.post<ChannelIQItem>(`/channel-iq/${channelId}/run`)
  return data
}

export default { list, run }
