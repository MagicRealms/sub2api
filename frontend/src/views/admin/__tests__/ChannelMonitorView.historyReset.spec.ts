import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import MonitorActionsCell from '@/components/admin/monitor/MonitorActionsCell.vue'
import ChannelMonitorView from '@/views/admin/ChannelMonitorView.vue'

const {
  listMonitors,
  resetHistory,
  showSuccess,
  showError,
} = vi.hoisted(() => ({
  listMonitors: vi.fn(),
  resetHistory: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))


vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorV1Mode: () => true,
  isChannelMonitorV2Mode: () => false,
  getChannelMonitorMode: () => 'v1' as const,
}))

vi.mock('@/features/channel-monitor-v2/MonitorSettingsPanel.vue', () => ({
  default: { name: 'MonitorSettingsPanel', template: '<div data-testid="v2-settings" />' },
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channelMonitor: {
      list: listMonitors,
      duplicate: vi.fn(),
      resetHistory,
      update: vi.fn(),
      runNow: vi.fn(),
      del: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const AppLayoutStub = defineComponent({
  template: '<main><slot /></main>',
})

const TablePageLayoutStub = defineComponent({
  template: '<section><slot name="filters" /><slot name="table" /><slot name="pagination" /></section>',
})

const DataTableStub = defineComponent({
  props: {
    data: { type: Array, default: () => [] },
    columns: { type: Array, default: () => [] },
    loading: { type: Boolean, default: false },
  },
  template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>',
})

function makeMonitor(overrides: Partial<ChannelMonitor> = {}): ChannelMonitor {
  return {
    id: 42,
    name: 'primary',
    provider: 'openai',
    api_mode: 'chat_completions',
    endpoint: 'https://api.example.com',
    api_key_masked: 'sk-t***',
    primary_model: 'gpt-4o-mini',
    extra_models: [],
    group_name: '',
    enabled: true,
    interval_seconds: 60,
    jitter_seconds: 0,
    last_checked_at: null,
    created_by: 1,
    created_at: '2026-07-16T00:00:00Z',
    updated_at: '2026-07-16T00:00:00Z',
    primary_status: '',
    primary_latency_ms: null,
    availability_7d: 0,
    extra_models_status: [],
    template_id: null,
    extra_headers: {},
    body_override_mode: 'off',
    body_override: null,
    check_mode: 'probe',
    account_id: null,
    ...overrides,
  }
}

const monitor = makeMonitor()

function mountView() {
  return mount(ChannelMonitorView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        MonitorFiltersBar: true,
        Pagination: true,
        ConfirmDialog: true,
        EmptyState: true,
        HelpTooltip: true,
        Toggle: true,
        MonitorFormDialog: true,
        MonitorTemplateManagerDialog: true,
        MonitorRunResultDialog: true,
        MonitorPrimaryModelCell: true,
      },
    },
  })
}

describe('ChannelMonitorView history reset', () => {
  beforeEach(() => {
    for (const fn of [listMonitors, resetHistory, showSuccess, showError]) fn.mockReset()
    listMonitors.mockResolvedValue({ items: [monitor], total: 1, page: 1, page_size: 20, pages: 1 })
    resetHistory.mockResolvedValue({ monitors_reset: 1, history_deleted: 3, rollups_deleted: 1 })
  })

  it('requires confirmation, targets only the selected monitor, and refreshes status', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="monitor-reset-history"]').trigger('click')
    await flushPromises()
    expect(resetHistory).not.toHaveBeenCalled()
    const dialog = wrapper.findAllComponents({ name: 'ConfirmDialog' }).find(d => d.props('show'))!
    expect(dialog.props('message')).toBe('admin.channelMonitor.resetHistoryConfirm')
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(resetHistory).toHaveBeenCalledWith(42, expect.stringContaining('channel-monitor-history-reset-'))
    expect(showSuccess).toHaveBeenCalledWith('admin.channelMonitor.resetHistorySuccess')
    expect(listMonitors).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('cancels without clearing history', async () => {
    const wrapper = mountView()
    await flushPromises()
    wrapper.findComponent(MonitorActionsCell).vm.$emit('reset-history', monitor)
    await flushPromises()
    const dialog = wrapper.findAllComponents({ name: 'ConfirmDialog' }).find(d => d.props('show'))!
    dialog.vm.$emit('cancel')
    await flushPromises()
    expect(resetHistory).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('explicitly confirms global reset and deduplicates clicks in flight', async () => {
    let finish!: (value: unknown) => void
    resetHistory.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const wrapper = mountView()
    await flushPromises()
    wrapper.findComponent({ name: 'MonitorFiltersBar' }).vm.$emit('reset-all-history')
    await flushPromises()
    const dialog = wrapper.findAllComponents({ name: 'ConfirmDialog' }).find(d => d.props('show'))!
    expect(dialog.props('message')).toBe('admin.channelMonitor.resetAllHistoryConfirm')
    dialog.vm.$emit('confirm')
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(resetHistory).toHaveBeenCalledTimes(1)
    expect(resetHistory.mock.calls[0][0]).toBeNull()
    finish({ monitors_reset: 14, history_deleted: 100, rollups_deleted: 2 })
    await flushPromises()
    wrapper.unmount()
  })

  it('keeps the same operation key after ambiguous network failure', async () => {
    resetHistory.mockRejectedValueOnce(new Error('Network error'))
    const wrapper = mountView()
    await flushPromises()
    wrapper.findComponent(MonitorActionsCell).vm.$emit('reset-history', monitor)
    await flushPromises()
    const dialog = wrapper.findAllComponents({ name: 'ConfirmDialog' }).find(d => d.props('show'))!
    dialog.vm.$emit('confirm')
    await flushPromises()
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(resetHistory.mock.calls[0][1]).toBe(resetHistory.mock.calls[1][1])
    wrapper.unmount()
  })

  it('shows a localized busy message and permits a fresh retry', async () => {
    resetHistory.mockRejectedValueOnce({ status: 409, reason: 'MONITOR_CHECK_RUNNING' })
    const wrapper = mountView()
    await flushPromises()
    wrapper.findComponent(MonitorActionsCell).vm.$emit('reset-history', monitor)
    await flushPromises()
    const dialog = wrapper.findAllComponents({ name: 'ConfirmDialog' }).find(d => d.props('show'))!
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('admin.channelMonitor.resetHistoryBusy')
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(resetHistory.mock.calls[0][1]).not.toBe(resetHistory.mock.calls[1][1])
    wrapper.unmount()
  })

  it('preserves the operation key when a previous request is still processing', async () => {
    resetHistory.mockRejectedValueOnce({ status: 409, reason: 'IDEMPOTENCY_IN_PROGRESS' })
    const wrapper = mountView()
    await flushPromises()
    wrapper.findComponent(MonitorActionsCell).vm.$emit('reset-history', monitor)
    await flushPromises()
    const dialog = wrapper.findAllComponents({ name: 'ConfirmDialog' }).find(d => d.props('show'))!
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('admin.channelMonitor.resetHistoryPending')
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(resetHistory.mock.calls[0][1]).toBe(resetHistory.mock.calls[1][1])
    wrapper.unmount()
  })
})
