<template>
  <component :is="authStore.isAuthenticated ? AppLayout : 'div'" class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <header v-if="!authStore.isAuthenticated" class="border-b border-gray-200 bg-white dark:border-dark-800 dark:bg-dark-900">
      <div class="mx-auto flex max-w-6xl items-center justify-between gap-3 px-4 py-4 sm:px-6">
        <RouterLink to="/home" class="flex min-w-0 items-center gap-2.5 font-semibold text-gray-900 dark:text-white">
          <img :src="siteLogo || '/logo.svg'" alt="" class="h-9 w-9 shrink-0 rounded-xl" />
          <span class="truncate">{{ appStore.siteName }}</span>
        </RouterLink>
        <div class="flex shrink-0 items-center gap-3">
          <LocaleSwitcher />
          <RouterLink :to="{ path: '/login', query: { redirect: '/guide' } }" class="btn btn-primary text-sm">{{ t('usageGuide.login') }}</RouterLink>
        </div>
      </div>
    </header>

    <div class="guide-page mx-auto max-w-6xl space-y-7" :class="!authStore.isAuthenticated && 'px-4 py-6 sm:px-6 sm:py-8'">
      <section class="relative overflow-hidden rounded-2xl border border-primary-200/70 bg-white dark:border-primary-900 dark:bg-dark-900" aria-labelledby="guide-title">
        <div class="grid gap-8 p-6 sm:p-8 xl:grid-cols-[1.4fr_1fr] xl:items-center xl:p-10">
          <div>
            <div class="mb-5 inline-flex items-center gap-2 rounded-full bg-primary-50 px-3 py-1.5 text-xs font-semibold text-primary-700 dark:bg-primary-950 dark:text-primary-300">
              <span class="h-1.5 w-1.5 rounded-full bg-primary-500"></span>
              {{ t('usageGuide.eyebrow') }}
            </div>
            <h1 id="guide-title" class="text-3xl font-bold leading-tight tracking-tight text-gray-950 dark:text-white sm:text-4xl">{{ t('usageGuide.title') }}<br /><span class="text-primary-600 dark:text-primary-400">{{ t('usageGuide.titleAccent') }}</span></h1>
            <p class="mt-4 max-w-lg text-sm leading-7 text-gray-600 dark:text-dark-300">{{ t('usageGuide.description', { site: appStore.siteName }) }}</p>
            <div class="mt-6 flex flex-wrap gap-3">
              <RouterLink to="/keys" class="btn btn-primary gap-2"><Icon name="key" size="sm" />{{ t('usageGuide.getKey') }}<Icon name="arrowRight" size="sm" /></RouterLink>
              <a href="#clients" class="btn btn-secondary">{{ t('usageGuide.chooseClient') }}</a>
            </div>
            <p class="mt-4 text-xs text-gray-500 dark:text-dark-400">{{ t('usageGuide.heroNote') }}</p>
          </div>
          <div class="rounded-xl bg-slate-950 p-5 font-mono text-xs shadow-xl shadow-slate-900/10 sm:p-6" :aria-label="t('usageGuide.previewLabel')">
            <div class="mb-6 flex items-center justify-between text-slate-500"><span class="flex gap-1.5" aria-hidden="true"><i class="h-2 w-2 rounded-full bg-rose-400"></i><i class="h-2 w-2 rounded-full bg-amber-300"></i><i class="h-2 w-2 rounded-full bg-emerald-400"></i></span><span>hello, world</span></div>
            <div class="space-y-4 leading-6">
              <p class="text-slate-500">{{ t('usageGuide.previewComment') }}</p>
              <p class="text-slate-200"><span class="text-primary-400">01</span> &nbsp; API Key <span class="float-right text-slate-500">sk-••••••••</span></p>
              <p class="text-slate-200"><span class="text-primary-400">02</span> &nbsp; Base URL <span class="float-right text-slate-500">/v1</span></p>
              <p class="text-slate-200"><span class="text-primary-400">03</span> &nbsp; Model <span class="float-right text-slate-500">gpt-6.1-sol</span></p>
              <div class="border-t border-white/10 pt-4"><p class="text-slate-400">&gt; {{ t('usageGuide.testPrompt') }}</p><p class="mt-2 flex items-center gap-2 text-emerald-300"><Icon name="check" size="sm" />{{ t('usageGuide.previewReply') }}</p></div>
            </div>
          </div>
        </div>
      </section>

      <nav class="flex flex-wrap gap-x-6 gap-y-3 text-sm text-gray-500 dark:text-dark-300" :aria-label="t('usageGuide.contents')">
        <a v-for="(section, index) in sections" :key="section" :href="`#${section}`" class="flex items-center gap-2 hover:text-primary-600"><span class="font-mono text-xs text-primary-600">0{{ index + 1 }}</span>{{ t(`usageGuide.sections.${section}`) }}<Icon name="chevronRight" size="xs" /></a>
      </nav>

      <section id="prepare" aria-labelledby="prepare-title">
        <div class="mb-4 flex flex-wrap items-baseline justify-between gap-2"><h2 id="prepare-title" class="guide-heading">{{ t('usageGuide.prepareTitle') }}</h2><span class="text-xs text-gray-500 dark:text-dark-400">{{ t('usageGuide.prepareHint') }}</span></div>
        <div class="grid gap-4 md:grid-cols-3">
          <article v-for="(step, index) in preparation" :key="step" class="guide-card p-5">
            <div class="mb-4 flex items-center justify-between"><span class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-50 font-mono text-sm font-semibold text-primary-700 dark:bg-primary-950 dark:text-primary-300">{{ index + 1 }}</span><Icon :name="step === 'account' ? 'user' : step === 'key' ? 'key' : 'cog'" size="md" class="text-gray-400" /></div>
            <h3 class="font-semibold text-gray-900 dark:text-white">{{ t(`usageGuide.preparation.${step}.title`) }}</h3>
            <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-300">{{ t(`usageGuide.preparation.${step}.text`) }}</p>
            <RouterLink :to="step === 'account' ? (authStore.isAuthenticated ? '/dashboard' : '/login') : '/keys'" class="mt-4 inline-flex items-center gap-1 text-sm font-medium text-primary-700 hover:text-primary-600 dark:text-primary-400">{{ t(`usageGuide.preparation.${step}.action`) }}<Icon name="arrowRight" size="sm" /></RouterLink>
          </article>
        </div>
        <div class="mt-4 flex gap-3 rounded-xl bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-800 dark:bg-amber-950/30 dark:text-amber-200"><Icon name="lightbulb" size="md" class="mt-0.5 shrink-0" /><p>{{ t('usageGuide.groupNote') }}</p></div>
      </section>

      <section id="clients" aria-labelledby="clients-title">
        <h2 id="clients-title" class="guide-heading">{{ t('usageGuide.clientsTitle') }}</h2>
        <p class="mb-4 mt-2 text-sm text-gray-500 dark:text-dark-300">{{ t('usageGuide.clientsHint') }}</p>
        <div class="grid grid-cols-2 gap-3 lg:grid-cols-4" :aria-label="t('usageGuide.chooseClient')">
          <button v-for="client in clients" :key="client.id" type="button" :aria-pressed="activeClient === client.id" :aria-controls="`guide-${client.id}`" class="rounded-xl border p-4 text-left transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500" :class="activeClient === client.id ? 'border-primary-500 bg-primary-50 ring-1 ring-primary-500 dark:bg-primary-950/40' : 'border-gray-200 bg-white hover:border-primary-300 dark:border-dark-700 dark:bg-dark-900'" :data-test="`client-${client.id}`" @click="activeClient = client.id">
            <span class="mb-3 flex items-center justify-between"><Icon :name="client.icon" size="md" :class="activeClient === client.id ? 'text-primary-600 dark:text-primary-400' : 'text-gray-400'" /><Icon v-if="activeClient === client.id" name="check" size="sm" class="text-primary-600" /></span>
            <span class="block text-sm font-semibold text-gray-900 dark:text-white">{{ client.name }}</span><span class="mt-1 block text-xs leading-5 text-gray-500 dark:text-dark-300">{{ t(`usageGuide.clientSummaries.${client.id}`) }}</span>
          </button>
        </div>

        <article :id="`guide-${activeClient}`" :key="activeClient" class="guide-card mt-4 overflow-hidden" :aria-label="activeClientName">
          <div class="flex flex-wrap items-center justify-between gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-800 sm:px-6">
            <div><h3 class="font-semibold text-gray-900 dark:text-white">{{ activeClientName }}</h3><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ activeClient === 'cherry' ? t('usageGuide.noTerminal') : 'OpenAI Responses API' }}</p></div>
            <div v-if="activeClient !== 'cherry'" class="flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800" :aria-label="t('usageGuide.os')">
              <button v-for="os in operatingSystems" :key="os" type="button" :aria-pressed="activeOs === os" class="rounded-md px-3 py-1.5 text-xs font-medium" :class="activeOs === os ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white' : 'text-gray-500 dark:text-dark-300'" :data-test="`os-${os}`" @click="activeOs = os">{{ os === 'windows' ? 'Windows · PowerShell' : 'macOS / Linux' }}</button>
            </div>
            <a v-else href="https://www.cherryai.com/" target="_blank" rel="noopener noreferrer" class="guide-text-link">{{ t('usageGuide.downloadCherry') }}<Icon name="externalLink" size="sm" /></a>
          </div>

          <div class="space-y-6 p-5 sm:p-6">
            <template v-if="activeClient === 'cherry'">
              <div><h4 class="guide-step-title"><span>1</span>{{ t('usageGuide.cherry.addTitle') }}</h4><p class="guide-step-text">{{ t('usageGuide.cherry.addText') }}</p></div>
              <div>
                <h4 class="guide-step-title"><span>2</span>{{ t('usageGuide.cherry.fieldsTitle') }}</h4>
                <div class="mt-3 divide-y divide-gray-100 rounded-xl border border-gray-200 dark:divide-dark-700 dark:border-dark-700">
                  <div v-for="field in cherryFields" :key="field.id" class="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"><span class="shrink-0 text-sm text-gray-500 dark:text-dark-300">{{ field.label }}</span><div class="flex min-w-0 items-center justify-between gap-3 sm:justify-end"><code class="break-all text-sm text-gray-900 dark:text-gray-100">{{ field.value }}</code><button v-if="field.copy" type="button" class="shrink-0 rounded p-1.5 text-gray-400 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700" :aria-label="t('usageGuide.copyCode', { label: field.label })" @click="copyToClipboard(field.value)"><Icon name="copy" size="sm" /></button></div></div>
                </div>
                <p class="mt-3 text-xs leading-6 text-gray-500 dark:text-dark-400">{{ t('usageGuide.cherry.urlNote') }}</p>
              </div>
              <div><h4 class="guide-step-title"><span>3</span>{{ t('usageGuide.cherry.chatTitle') }}</h4><p class="guide-step-text">{{ t('usageGuide.cherry.chatText') }}</p></div>
            </template>

            <template v-else-if="activeClient === 'codex'">
              <div><h4 class="guide-step-title"><span>1</span>{{ t('usageGuide.codex.installTitle') }}</h4><p class="guide-step-text">{{ t('usageGuide.codex.installText') }} <a href="https://nodejs.org/en/download" target="_blank" rel="noopener noreferrer" class="guide-text-link">Node.js LTS ↗</a></p><GuideCodeBlock class="mt-3" label="Terminal" code="npm install -g @openai/codex" /></div>
              <div class="rounded-xl bg-primary-50 p-4 dark:bg-primary-950/30"><p class="text-sm font-semibold text-primary-800 dark:text-primary-200">{{ t('usageGuide.codex.easyTitle') }}</p><p class="mt-1 text-sm leading-6 text-primary-700 dark:text-primary-300">{{ t('usageGuide.codex.easyText') }}</p><div class="mt-3 flex flex-wrap gap-4"><RouterLink to="/keys" class="guide-text-link">{{ t('usageGuide.getKey') }}<Icon name="arrowRight" size="sm" /></RouterLink><a href="https://github.com/farion1231/cc-switch/releases" target="_blank" rel="noopener noreferrer" class="guide-text-link">{{ t('usageGuide.codex.downloadSwitch') }}<Icon name="externalLink" size="sm" /></a></div></div>
              <details class="group rounded-xl border border-gray-200 dark:border-dark-700" data-test="codex-manual"><summary class="cursor-pointer px-4 py-3 text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('usageGuide.codex.manualTitle') }}</summary><div class="space-y-5 border-t border-gray-100 p-4 dark:border-dark-700"><p class="text-sm leading-6 text-gray-500 dark:text-dark-300">{{ t('usageGuide.codex.configText') }}</p><GuideCodeBlock :label="codexConfigPath" :code="codexConfig" /><p class="text-sm leading-6 text-gray-500 dark:text-dark-300">{{ t('usageGuide.codex.envText') }}</p><GuideCodeBlock :label="shellLabel" :code="`${keyEnvironment}\ncodex`" /><p class="text-xs leading-6 text-gray-500 dark:text-dark-400">{{ t('usageGuide.envNote') }}</p></div></details>
              <div><h4 class="guide-step-title"><span>2</span>{{ t('usageGuide.codex.startTitle') }}</h4><p class="guide-step-text">{{ t('usageGuide.codex.startText') }}</p></div>
            </template>

            <template v-else-if="activeClient === 'opencode'">
              <div><h4 class="guide-step-title"><span>1</span>{{ t('usageGuide.opencode.installTitle') }}</h4><p class="guide-step-text">{{ t('usageGuide.opencode.installText') }}</p><GuideCodeBlock class="mt-3" label="Terminal" code="npm install -g opencode-ai" /></div>
              <div><h4 class="guide-step-title"><span>2</span>{{ t('usageGuide.opencode.configTitle') }}</h4><p class="guide-step-text">{{ t('usageGuide.opencode.configText') }}</p><GuideCodeBlock class="mt-3" label="opencode.json" :code="openCodeConfig" /></div>
              <div><h4 class="guide-step-title"><span>3</span>{{ t('usageGuide.opencode.startTitle') }}</h4><p class="guide-step-text">{{ t('usageGuide.opencode.startText') }}</p><GuideCodeBlock class="mt-3" :label="shellLabel" :code="`${keyEnvironment}\nopencode`" /><p class="mt-3 text-xs leading-6 text-gray-500 dark:text-dark-400">{{ t('usageGuide.envNote') }}</p></div>
            </template>

            <template v-else>
              <div><h4 class="guide-step-title"><span>1</span>{{ t('usageGuide.api.modelsTitle') }}</h4><p class="guide-step-text">{{ t('usageGuide.api.modelsText') }}</p><GuideCodeBlock class="mt-3" :label="shellLabel" :code="modelsCommand" /></div>
              <div><h4 class="guide-step-title"><span>2</span>{{ t('usageGuide.api.requestTitle') }}</h4><p class="guide-step-text">{{ t('usageGuide.api.requestText') }}</p><GuideCodeBlock class="mt-3" :label="shellLabel" :code="responseCommand" /></div>
              <p class="text-xs leading-6 text-gray-500 dark:text-dark-400">{{ t('usageGuide.api.billingNote') }}</p>
            </template>

            <div class="flex gap-2 border-t border-gray-100 pt-4 text-xs leading-6 text-gray-500 dark:border-dark-700 dark:text-dark-400"><Icon name="lightbulb" size="sm" class="mt-1 shrink-0" /><p>{{ t('usageGuide.modelNote') }}</p></div>
          </div>
        </article>
      </section>

      <section id="verify" class="guide-card p-5 sm:p-6" aria-labelledby="verify-title">
        <div class="grid gap-6 lg:grid-cols-[1fr_1.2fr] lg:items-center">
          <div><span class="mb-3 inline-flex h-9 w-9 items-center justify-center rounded-full bg-primary-50 text-primary-600 dark:bg-primary-950 dark:text-primary-400"><Icon name="check" size="md" /></span><h2 id="verify-title" class="guide-heading">{{ t('usageGuide.verifyTitle') }}</h2><p class="mt-2 text-sm leading-7 text-gray-500 dark:text-dark-300">{{ t('usageGuide.verifyText') }}</p><RouterLink to="/usage" class="guide-text-link mt-3">{{ t('usageGuide.viewUsage') }}<Icon name="arrowRight" size="sm" /></RouterLink></div>
          <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800"><p class="text-xs text-gray-500 dark:text-dark-400">{{ t('usageGuide.tryPrompt') }}</p><div class="mt-3 flex items-start justify-between gap-3"><p class="text-sm leading-7 text-gray-800 dark:text-gray-100">{{ t('usageGuide.testPrompt') }}</p><button type="button" class="shrink-0 rounded-md p-1.5 text-gray-400 hover:text-primary-600" :aria-label="t('usageGuide.copyPrompt')" @click="copyToClipboard(t('usageGuide.testPrompt'))"><Icon name="copy" size="sm" /></button></div><p class="mt-3 border-t border-gray-200 pt-3 text-xs leading-6 text-gray-500 dark:border-dark-700 dark:text-dark-400">{{ t('usageGuide.verifyNote') }}</p></div>
        </div>
      </section>

      <section id="faq" aria-labelledby="faq-title">
        <h2 id="faq-title" class="guide-heading mb-4">{{ t('usageGuide.faqTitle') }}</h2>
        <div class="guide-card divide-y divide-gray-100 dark:divide-dark-800"><details v-for="question in questions" :key="question" class="group px-5 sm:px-6"><summary class="flex cursor-pointer list-none items-center justify-between gap-4 py-4 text-sm font-medium text-gray-800 dark:text-gray-100"><span>{{ t(`usageGuide.faq.${question}.q`) }}</span><Icon name="chevronDown" size="sm" class="shrink-0 text-gray-400 transition-transform group-open:rotate-180" /></summary><p class="pb-5 text-sm leading-7 text-gray-500 dark:text-dark-300">{{ t(`usageGuide.faq.${question}.a`) }}</p></details></div>
        <p class="mt-4 text-xs leading-6 text-gray-500 dark:text-dark-400">{{ t('usageGuide.supportNote') }}</p>
      </section>

      <footer class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-200 pb-2 pt-5 text-xs text-gray-400 dark:border-dark-800 dark:text-dark-400"><span>{{ t('usageGuide.sourcesNote') }}</span><div class="flex flex-wrap gap-4"><a v-for="source in sources" :key="source.name" :href="source.url" target="_blank" rel="noopener noreferrer" class="hover:text-primary-600">{{ source.name }} ↗</a></div></footer>
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import GuideCodeBlock from '@/components/common/GuideCodeBlock.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo, { allowRelative: true, allowDataUrl: true }))
const sections = ['prepare', 'clients', 'verify', 'faq'] as const
const preparation = ['account', 'key', 'config'] as const
const questions = ['key', 'model', 'auth', 'notFound', 'rate', 'balance', 'context', 'website'] as const
const clients = [
  { id: 'cherry', name: 'Cherry Studio', icon: 'chat' },
  { id: 'codex', name: 'Codex', icon: 'cube' },
  { id: 'opencode', name: 'OpenCode', icon: 'terminal' },
  { id: 'api', name: 'API / cURL', icon: 'link' }
] as const
const activeClient = ref<(typeof clients)[number]['id']>('cherry')
const activeClientName = computed(() => clients.find(client => client.id === activeClient.value)?.name)
const operatingSystems = ['unix', 'windows'] as const
const activeOs = ref<(typeof operatingSystems)[number]>(/Win/i.test(navigator.userAgent) ? 'windows' : 'unix')
const shellLabel = computed(() => activeOs.value === 'windows' ? 'PowerShell' : 'Terminal · bash / zsh')
// Follow the configured public API address, including any deployment path prefix.
const baseRoot = computed(() => {
  const value = sanitizeUrl(appStore.apiBaseUrl) || window.location.origin
  const url = new URL(value)
  return `${url.origin}${url.pathname.replace(/\/+$/, '').replace(/\/v1$/, '')}`
})
const apiBase = computed(() => `${baseRoot.value}/v1`)
const exampleModel = 'gpt-6.1-sol'
const keyPlaceholder = 'YOUR_API_KEY'
const cherryFields = computed(() => [
  { id: 'name', label: t('usageGuide.fields.name'), value: appStore.siteName, copy: false },
  { id: 'type', label: t('usageGuide.fields.type'), value: 'OpenAI', copy: false },
  { id: 'url', label: 'API Host / Base URL', value: baseRoot.value, copy: true },
  { id: 'key', label: 'API Key', value: t('usageGuide.fields.keyValue'), copy: false },
  { id: 'model', label: 'Model ID', value: exampleModel, copy: true }
])
const codexConfigPath = computed(() => activeOs.value === 'windows' ? '%USERPROFILE%\\.codex\\config.toml' : '~/.codex/config.toml')
const codexConfig = computed(() => `model_provider = "sub2api"
model = "${exampleModel}"

[model_providers.sub2api]
name = "Sub2API"
base_url = ${JSON.stringify(apiBase.value)}
wire_api = "responses"
env_key = "SUB2API_API_KEY"
requires_openai_auth = false`)
const keyEnvironment = computed(() => activeOs.value === 'windows'
  ? `$env:SUB2API_API_KEY="${keyPlaceholder}"`
  : `export SUB2API_API_KEY="${keyPlaceholder}"`)
const openCodeConfig = computed(() => JSON.stringify({
  $schema: 'https://opencode.ai/config.json',
  model: `sub2api/${exampleModel}`,
  provider: { sub2api: {
    npm: '@ai-sdk/openai', name: 'Sub2API',
    options: { baseURL: apiBase.value, apiKey: '{env:SUB2API_API_KEY}' },
    models: { [exampleModel]: { name: exampleModel } }
  } }
}, null, 2))
const modelsCommand = computed(() => activeOs.value === 'windows'
  ? `${keyEnvironment.value}\nInvoke-RestMethod -Uri "${apiBase.value}/models" -Headers @{ Authorization = "Bearer $env:SUB2API_API_KEY" }`
  : `${keyEnvironment.value}\ncurl '${apiBase.value}/models' \\\n+  -H "Authorization: Bearer $SUB2API_API_KEY"`)
const responseCommand = computed(() => activeOs.value === 'windows'
  ? `$body = @{ model = "${exampleModel}"; input = "Say hello in one sentence."; stream = $true } | ConvertTo-Json -Compress\n$body | curl.exe -N "${apiBase.value}/responses" -H "Authorization: Bearer $env:SUB2API_API_KEY" -H "Content-Type: application/json" --data-binary '@-'`
  : `curl -N '${apiBase.value}/responses' \\\n+  -H "Authorization: Bearer $SUB2API_API_KEY" \\\n+  -H 'Content-Type: application/json' \\\n+  -d '{"model":"${exampleModel}","input":"Say hello in one sentence.","stream":true}'`)
const sources = [
  { name: 'OpenAI Docs', url: 'https://developers.openai.com/codex/config-advanced/' },
  { name: 'Cherry Studio', url: 'https://www.cherryai.com/docs/en/pre-basic/settings/providers/' },
  { name: 'OpenCode', url: 'https://opencode.ai/docs/providers/' }
]
</script>

<style scoped>
.guide-page { color-scheme: light dark; }
.guide-page section[id] { scroll-margin-top: 5rem; }
.guide-heading { @apply text-lg font-semibold tracking-tight text-gray-900 dark:text-white; }
.guide-card { @apply rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900; }
.guide-text-link { @apply inline-flex items-center gap-1 text-sm font-medium text-primary-700 hover:text-primary-600 dark:text-primary-400; }
.guide-step-title { @apply flex items-center gap-2.5 text-sm font-semibold text-gray-900 dark:text-white; }
.guide-step-title > span { @apply flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-gray-100 font-mono text-xs text-gray-500 dark:bg-dark-700 dark:text-dark-300; }
.guide-step-text { @apply mt-2 text-sm leading-7 text-gray-500 dark:text-dark-300; }
summary::-webkit-details-marker { display: none; }
</style>
