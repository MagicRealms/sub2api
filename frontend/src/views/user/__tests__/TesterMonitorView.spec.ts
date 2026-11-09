import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import TesterMonitorView from '../TesterMonitorView.vue'

const api=vi.hoisted(()=>({ snapshot:vi.fn(),query:vi.fn(),keys:vi.fn(),errors:vi.fn() }))
vi.mock('@/api/tester',()=>({ getTesterSnapshot:api.snapshot }))
vi.mock('@/api',()=>({usageAPI:{query:api.query},keysAPI:{list:api.keys}}))
vi.mock('@/api/client',()=>({apiClient:{get:api.errors}}))
vi.mock('vue-i18n',async importOriginal=>({...(await importOriginal<typeof import('vue-i18n')>()),useI18n:()=>({t:(key:string)=>key})}))
const metric={is_total:true,model:'gpt-6.1-sol',group_id:6,group_name:'Enterprise',requests:1,input_tokens:100,output_tokens:10,cache_read_tokens:100,cache_creation_tokens:0,total_cost:1,actual_cost:.32,cache_hit_rate:50,first_token_samples:1,output_samples:1,first_token_p50_ms:1000,first_token_p95_ms:1000,duration_p50_ms:2000,duration_p95_ms:2000,output_tokens_per_second:10,cost_per_million_tokens:1523.8}
const snapshot={generated_at:'2026-11-09T10:00:00Z',visible_after:null,summary:metric,models:[metric],models_truncated:false,error_records:0}
const mounted:ReturnType<typeof mount>[]=[]
function render() {
  const w=mount(TesterMonitorView,{global:{stubs:{
    AppLayout:{template:'<div><slot/></div>'},RouterLink:true,Select:true,
    BaseDialog:{props:['show'],template:'<div v-if="show"><slot/></div>'},TesterHistoryResetButton:true,
  }}})
  mounted.push(w);return w
}
beforeEach(()=>{
  vi.clearAllMocks()
  api.snapshot.mockResolvedValue(snapshot)
  api.query.mockResolvedValue({items:[],total:0})
  api.keys.mockResolvedValue({items:[],total:0})
  api.errors.mockResolvedValue({data:{items:[],total:0}})
})
afterEach(()=>{mounted.splice(0).forEach(w=>w.unmount());vi.useRealTimers()})
describe('personal tester monitoring',()=>{
  it('loads on entry, never polls, and refreshes only on explicit interaction',async()=>{
    const w=render();await flushPromises()
    expect(api.snapshot).toHaveBeenCalledTimes(1)
    vi.useFakeTimers();await vi.advanceTimersByTimeAsync(120000)
    expect(api.snapshot).toHaveBeenCalledTimes(1)
    await w.findAll('button').find(b=>b.text()==='tester.refresh')!.trigger('click');await flushPromises()
    expect(api.snapshot).toHaveBeenCalledTimes(2)
  })
  it('scopes a selected model to its group and shows that filter',async()=>{
    const w=render();await flushPromises()
    await w.findAll('button').find(b=>b.text().includes('gpt-6.1-sol'))!.trigger('click');await flushPromises()
    expect(api.snapshot).toHaveBeenLastCalledWith(expect.objectContaining({model:'gpt-6.1-sol',group_id:6}),expect.any(AbortSignal))
    expect(w.text()).toContain('tester.group：Enterprise')
  })
  it('cancels the previous request on refresh and on navigation',async()=>{
    const w=render();await flushPromises()
    const first=api.snapshot.mock.calls[0][1] as AbortSignal
    await w.findAll('button').find(b=>b.text()==='tester.refresh')!.trigger('click');await flushPromises()
    expect(first.aborted).toBe(true)
    const second=api.snapshot.mock.calls[1][1] as AbortSignal
    w.unmount();expect(second.aborted).toBe(true)
    mounted.splice(mounted.indexOf(w),1)
  })
  it('labels retained data as stale when the next load fails',async()=>{
    const w=render();await flushPromises()
    api.snapshot.mockRejectedValueOnce(new Error('unavailable'))
    await w.findAll('button').find(b=>b.text()==='tester.refresh')!.trigger('click');await flushPromises()
    expect(w.get('[role="alert"]').text()).toBe('tester.loadFailed')
    expect(w.text()).toContain('$0.320000')
  })
})
