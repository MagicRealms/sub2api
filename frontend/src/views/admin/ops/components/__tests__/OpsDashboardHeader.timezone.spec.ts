import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import OpsDashboardHeader from '../OpsDashboardHeader.vue'

vi.mock('@/api', () => ({ adminAPI: { groups: { getAllIncludingInactive: vi.fn().mockResolvedValue([]) } } }))
vi.mock('@/api/admin/ops', () => ({ opsAPI: { getRealtimeTrafficSummary: vi.fn().mockResolvedValue({ enabled: true, summary: null }) } }))
vi.mock('@/stores', () => ({ useAdminSettingsStore: () => ({ opsRealtimeMonitoringEnabled: true, setOpsRealtimeMonitoringEnabledLocal: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))
enableAutoUnmount(afterEach)
beforeEach(() => vi.stubEnv('TZ', 'America/Los_Angeles'))
afterEach(() => { vi.useRealTimers(); vi.unstubAllEnvs() })

async function openCustomRange(now: Date) {
  vi.useFakeTimers()
  vi.setSystemTime(now)
  const wrapper = mount(OpsDashboardHeader, {
    props: { platform: '', groupId: null, timeRange: '1h', queryMode: 'auto', loading: false, lastUpdated: null },
    global: { stubs: {
      Select: true, HelpTooltip: true, Icon: true,
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }
    } }
  })
  await flushPromises()
  const selector = wrapper.findAllComponents({ name: 'Select' }).find(select =>
    select.props('options').some((option: { value: unknown }) => option.value === 'custom')
  )!
  selector.vm.$emit('update:modelValue', 'custom')
  await flushPromises()
  return wrapper
}

describe('ops custom time range timezone', () => {
  it.each([
    [14, 23, '2026-10-09T13:23', '2026-10-09T14:23'],
    [0, 15, '2026-10-08T23:15', '2026-10-09T00:15']
  ] as const)('prefills the previous Beijing hour in a Pacific browser at %i:%i', async (hour, minute, start, end) => {
    const wrapper = await openCustomRange(new Date(`2026-10-09T${String(hour).padStart(2,'0')}:${String(minute).padStart(2,'0')}:00+08:00`))
    const inputs = wrapper.findAll<HTMLInputElement>('input[type="datetime-local"]')
    expect(inputs.map(input => input.element.value)).toEqual([start, end])
  })

  it('submits the previous real hour without applying the timezone offset twice', async () => {
    const now = new Date('2026-10-09T14:23:00+08:00')
    const wrapper = await openCustomRange(now)
    await wrapper.findAll('button').find(button => button.text() === 'common.confirm')!.trigger('click')
    expect(wrapper.emitted('update:customTimeRange')).toEqual([
      [new Date(now.getTime() - 3600000).toISOString(), now.toISOString()]
    ])
  })
})
