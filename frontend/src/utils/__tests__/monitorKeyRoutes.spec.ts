import { describe, expect, it } from 'vitest'
import type { ApiKey, Group } from '@/types'
import { monitorKeyGroups, monitorKeySupportsProvider, monitorRequestURL } from '../monitorKeyRoutes'

const groups = [
  { id: 1, name: 'GPT', platform: 'openai' },
  { id: 2, name: 'Claude', platform: 'anthropic' },
  { id: 3, name: 'Mixed', platform: 'composite' },
] as Group[]
const key = { group_id: 1, group: groups[0], group_ids: [1, 2] } as ApiKey

describe('monitor key routes', () => {
  it('accepts a provider in a secondary group without changing saved order', () => {
    expect(monitorKeySupportsProvider(key, groups, 'anthropic')).toBe(true)
    expect(monitorKeyGroups(key, groups).map(group => group.id)).toEqual([1, 2])
    expect(monitorKeySupportsProvider(key, groups, 'gemini')).toBe(false)
  })
  it('preserves primary-only keys and composite routes', () => {
    expect(monitorKeySupportsProvider({ ...key, group_ids: undefined }, [], 'openai')).toBe(true)
    expect(monitorKeySupportsProvider({ ...key, group_ids: [3] }, groups, 'anthropic')).toBe(true)
  })
  it('deduplicates saved routes and does not invent unknown group capabilities', () => {
    expect(monitorKeyGroups({ ...key, group_ids: [2, 1, 2, 999] }, groups).map(group => group.id)).toEqual([2, 1])
    expect(monitorKeySupportsProvider({ ...key, group_ids: [999] }, groups, 'openai')).toBe(false)
  })
})

describe('monitor request URL', () => {
  it.each([
    ['https://relay.example', 'anthropic', '', 'https://relay.example/v1/messages'],
    ['https://relay.example/v1/', 'anthropic', '', 'https://relay.example/v1/messages'],
    ['https://relay.example/anthropic/v1', 'anthropic', '', 'https://relay.example/anthropic/v1/messages'],
    ['https://relay.example/v1', 'openai', 'responses', 'https://relay.example/v1/responses'],
    ['https://relay.example', 'openai', 'chat_completions', 'https://relay.example/v1/chat/completions'],
    ['https://relay.example/v1beta', 'gemini', '', 'https://relay.example/v1beta/models/{model}:generateContent'],
    ['https://v1', 'anthropic', '', 'https://v1/v1/messages'],
    ['https://relay.example/tenant%2Fv1', 'anthropic', '', 'https://relay.example/tenant%2Fv1/v1/messages'],
    ['https://open.bigmodel.cn', 'zhipu', '', 'https://open.bigmodel.cn/api/paas/v4/chat/completions'],
    ['https://api.z.ai/api/coding/paas/v4', 'zhipu', '', 'https://api.z.ai/api/coding/paas/v4/chat/completions'],
    ['https://relay.example/v1', 'zhipu', '', 'https://relay.example/v1/chat/completions'],
    ['', 'anthropic', '', ''],
  ])('matches the backend for %s %s %s', (endpoint, provider, mode, expected) => {
    expect(monitorRequestURL(endpoint, provider, mode)).toBe(expected)
  })
})
