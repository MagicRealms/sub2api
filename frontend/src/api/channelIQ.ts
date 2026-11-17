import { apiClient } from './client'

export type ChannelIQStatus = 'success' | 'failed' | 'running'
export type ChannelIQCriterionStatus = 'pass' | 'partial' | 'fail' | 'unknown'

export interface ChannelIQCriterion {
  status: ChannelIQCriterionStatus
}

export interface ChannelIQEvaluation {
  graded: boolean
  grade: string
  score: number
  criteria: {
    wheels: ChannelIQCriterion
    leg_pedal: ChannelIQCriterion
    view_consistent: ChannelIQCriterion
    pelican_beak: ChannelIQCriterion
  }
}

export interface ChannelIQResult {
  id: number
  status: ChannelIQStatus | string
  model_id: string
  reasoning_effort: string
  note: string
  response_text: string
  error_message?: string
  latency_ms: number
  started_at: string
  finished_at: string
  created_at: string
  evaluation?: ChannelIQEvaluation
}

export interface ChannelIQPublicSnapshot {
  visible: boolean
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
