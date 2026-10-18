<template>
  <AppLayout>
    <div class="mx-auto w-full min-w-0 max-w-7xl space-y-5 pb-8">
      <header class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
              <span class="flex h-9 w-9 items-center justify-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400"><Icon name="play" size="sm" /></span>
              {{ t('testCenter.title') }}
            </h1>
            <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('testCenter.description') }}</p>
          </div>
          <RouterLink to="/keys" class="btn btn-secondary btn-sm"><Icon name="key" size="sm" />{{ t('testCenter.manageKeys') }}</RouterLink>
        </div>
        <p class="mt-4 rounded-xl bg-primary-50 px-4 py-3 text-xs leading-5 text-primary-800 dark:bg-primary-900/20 dark:text-primary-200">{{ t('testCenter.billingHint') }}</p>
      </header>

      <div class="grid min-w-0 items-start gap-5 lg:grid-cols-[290px_minmax(0,1fr)]">
        <aside class="min-w-0 space-y-4 rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="flex items-center justify-between gap-2">
            <h2 class="font-bold text-gray-900 dark:text-white">{{ t('testCenter.connection') }}</h2>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingKeys || loadingModels || busy" @click="loadKeys"><Icon name="refresh" size="sm" :class="loadingKeys ? 'animate-spin' : ''" />{{ t('testCenter.refresh') }}</button>
          </div>
          <div v-if="keysError" role="alert" class="test-error">{{ keysError }}</div>
          <div>
            <label for="test-key" class="mb-2 block text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('testCenter.apiKey') }}</label>
            <select id="test-key" v-model="selectedKeyId" class="input w-full min-w-0" :disabled="loadingKeys || busy" data-test="test-key">
              <option value="" disabled>{{ loadingKeys ? t('testCenter.loadingKeys') : t('testCenter.selectKey') }}</option>
              <option v-for="key in keys" :key="key.id" :value="String(key.id)" :disabled="!!keyUnavailableReason(key)">
                {{ key.name }} · #{{ key.id }}{{ keyUnavailableReason(key) ? ` · ${t(`testCenter.${keyUnavailableReason(key)}`)}` : '' }}
              </option>
            </select>
            <p v-if="!loadingKeys && !keysError && !usableKeys.length" class="mt-3 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('testCenter.noKeys') }} <RouterLink to="/keys" class="text-primary-600 hover:underline dark:text-primary-400">{{ t('testCenter.manageKeys') }}</RouterLink></p>
          </div>
          <dl v-if="selectedKey" class="space-y-3 rounded-2xl bg-gray-50 p-3 text-xs dark:bg-dark-700/70" data-test="test-key-info">
            <div class="flex gap-2"><dt class="shrink-0 text-gray-500 dark:text-gray-400">{{ t('testCenter.group') }}</dt><dd class="min-w-0 flex-1 break-words text-right font-semibold text-gray-800 dark:text-gray-100">{{ selectedKey.group?.name }}</dd></div>
            <div class="flex gap-2"><dt class="shrink-0 text-gray-500 dark:text-gray-400">{{ t('testCenter.quota') }}</dt><dd class="text-right font-semibold text-gray-800 dark:text-gray-100">{{ selectedKey.quota > 0 ? `$${Math.max(0, selectedKey.quota - selectedKey.quota_used).toFixed(2)}` : t('testCenter.unlimited') }}</dd></div>
          </dl>
          <div v-if="modelsError" role="alert" class="test-error">
            <p>{{ modelsError }}</p>
            <button type="button" class="mt-2 font-semibold underline" :disabled="busy" @click="loadModels">{{ t('testCenter.retry') }}</button>
          </div>
          <div>
            <label for="test-model" class="mb-2 block text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('testCenter.model') }}</label>
            <select id="test-model" v-model="selectedModel" class="input w-full min-w-0" :disabled="!selectedKey || loadingModels || busy || !modelOptions.length" data-test="test-model">
              <option value="" disabled>{{ loadingModels ? t('testCenter.loadingModels') : t('testCenter.selectModel') }}</option>
              <option v-for="model in modelOptions" :key="model" :value="model">{{ model }}</option>
            </select>
            <p class="mt-2 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('testCenter.modelsHint') }}</p>
          </div>
          <p class="border-t border-gray-100 pt-4 text-xs leading-5 text-gray-500 dark:border-dark-700 dark:text-gray-400">{{ t('testCenter.sessionHint') }}</p>
          <RouterLink to="/usage" class="inline-flex items-center gap-1 text-xs font-semibold text-primary-600 hover:underline dark:text-primary-400">{{ t('testCenter.viewUsage') }}<Icon name="arrowRight" size="sm" /></RouterLink>
        </aside>

        <section class="min-w-0 overflow-hidden rounded-3xl bg-white shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 p-4 dark:border-dark-700 sm:px-5">
            <div class="tabs inline-flex" role="tablist" :aria-label="t('testCenter.tabsLabel')">
              <button id="test-chat-tab" type="button" role="tab" aria-controls="test-chat-panel" :aria-selected="tab === 'chat'" class="tab" :class="{ 'tab-active': tab === 'chat' }" :disabled="busy" @click="tab = 'chat'">{{ t('testCenter.chatTab') }}</button>
              <button id="test-image-tab" type="button" role="tab" aria-controls="test-image-panel" :aria-selected="tab === 'image'" class="tab" :class="{ 'tab-active': tab === 'image' }" :disabled="busy" @click="tab = 'image'">{{ t('testCenter.imageTab') }}</button>
            </div>
            <button type="button" class="text-xs font-medium text-gray-500 hover:text-red-500 dark:text-gray-400" :disabled="busy" @click="tab === 'chat' ? clearChat() : clearImages()">{{ tab === 'chat' ? t('testCenter.clearChat') : t('testCenter.clearImages') }}</button>
          </div>

          <div v-if="tab === 'chat'" id="test-chat-panel" role="tabpanel" aria-labelledby="test-chat-tab">
            <div ref="transcript" class="h-[440px] min-h-[260px] space-y-4 overflow-y-auto p-4 sm:p-5" role="log" :aria-label="t('testCenter.conversation')" aria-live="polite" data-test="test-conversation">
              <div v-if="!messages.length" class="flex h-full flex-col items-center justify-center gap-3 text-center">
                <span class="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary-50 text-primary-500 dark:bg-primary-900/20"><Icon name="play" size="lg" /></span>
                <h2 class="font-bold text-gray-900 dark:text-white">{{ t('testCenter.chatWelcome') }}</h2>
                <p class="max-w-sm text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('testCenter.chatWelcomeHint') }}</p>
                <button type="button" class="rounded-full border border-gray-200 px-4 py-2 text-xs text-gray-600 hover:border-primary-300 dark:border-dark-600 dark:text-gray-300" @click="chatPrompt = t('testCenter.examplePrompt')">{{ t('testCenter.examplePrompt') }}</button>
              </div>
              <article v-for="message in messages" :key="message.id" class="max-w-[95%] rounded-2xl px-4 py-3 sm:max-w-[90%]" :class="message.role === 'user' ? 'ml-auto bg-primary-50 text-primary-950 dark:bg-primary-900/30 dark:text-primary-100' : 'bg-gray-50 text-gray-800 dark:bg-dark-700/70 dark:text-gray-100'" :data-test="`test-message-${message.role}`">
                <div class="mb-1.5 text-xs font-semibold" :class="message.role === 'user' ? 'text-primary-600 dark:text-primary-400' : 'text-gray-500 dark:text-gray-400'">{{ message.role === 'user' ? t('testCenter.you') : message.model }}</div>
                <div class="whitespace-pre-wrap break-words text-sm leading-7 [overflow-wrap:anywhere]">{{ message.content || (message.status === 'pending' ? t('testCenter.waitingReply') : '') }}</div>
                <div v-if="message.error" role="alert" class="mt-2 break-words text-xs text-red-600 dark:text-red-300">{{ message.error }}</div>
                <p v-if="message.status === 'stopped'" class="mt-2 text-xs text-amber-700 dark:text-amber-300">{{ t('testCenter.stopped') }}</p>
                <p v-if="message.result?.finishReason === 'length'" class="mt-2 text-xs text-amber-700 dark:text-amber-300">{{ t('testCenter.lengthLimit') }}</p>
                <div v-if="message.result" class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-gray-500 dark:text-gray-400">
                  <span>{{ t('testCenter.elapsed', { seconds: seconds(message.result.durationMs) }) }}</span>
                  <span v-if="message.result.firstTokenMs !== undefined">{{ t('testCenter.firstToken', { seconds: seconds(message.result.firstTokenMs) }) }}</span>
                  <span v-if="message.result.usage">{{ t('testCenter.tokens', { input: message.result.usage.inputTokens, output: message.result.usage.outputTokens }) }}</span>
                </div>
                <p v-if="message.requestId" class="mt-1 break-all text-[10px] text-gray-400">{{ t('testCenter.requestId') }}: {{ message.requestId }}</p>
              </article>
            </div>
            <form class="space-y-3 border-t border-gray-100 p-4 dark:border-dark-700 sm:p-5" @submit.prevent="sendChat">
              <p v-if="selectedKey && !loadingModels && !modelsError && !chatModels.length" class="text-xs text-amber-700 dark:text-amber-300">{{ t('testCenter.noChatModels') }}</p>
              <label for="test-chat-prompt" class="sr-only">{{ t('testCenter.chatPrompt') }}</label>
              <textarea id="test-chat-prompt" v-model="chatPrompt" rows="3" maxlength="16000" class="input w-full resize-y" :placeholder="t('testCenter.chatPlaceholder')" :disabled="busy" data-test="test-chat-prompt" @keydown="onPromptKeydown" />
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="text-xs text-gray-400">{{ t('testCenter.sendHint') }}</span>
                <button v-if="busy" type="button" class="btn btn-secondary" @click="stopRequest">{{ t('testCenter.stop') }}</button>
                <button v-else type="submit" class="btn btn-primary" :disabled="!canSend || !chatPrompt.trim()" data-test="test-chat-send"><Icon name="play" size="sm" />{{ t('testCenter.send') }}</button>
              </div>
            </form>
          </div>

          <div v-else id="test-image-panel" role="tabpanel" aria-labelledby="test-image-tab" class="space-y-5 p-4 sm:p-5">
            <div v-if="selectedKey && !imageAccess" class="rounded-xl bg-amber-50 p-4 text-sm leading-6 text-amber-800 dark:bg-amber-900/20 dark:text-amber-200" data-test="test-image-permission">{{ t('testCenter.imageNotAllowed') }}</div>
            <p v-else-if="selectedKey && !loadingModels && !modelsError && !imageModels.length" class="text-sm text-amber-700 dark:text-amber-300">{{ t('testCenter.noImageModels') }}</p>
            <form class="space-y-3" @submit.prevent="generateImage">
              <label for="test-image-prompt" class="block text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('testCenter.imagePrompt') }}</label>
              <textarea id="test-image-prompt" v-model="imagePrompt" class="input w-full resize-y" rows="4" maxlength="16000" :placeholder="t('testCenter.imagePlaceholder')" :disabled="busy" data-test="test-image-prompt" />
              <div class="flex flex-wrap items-center justify-between gap-3">
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ busy ? t('testCenter.imageWaiting') : t('testCenter.oneImageHint') }}</p>
                <button v-if="busy" type="button" class="btn btn-secondary" @click="stopRequest">{{ t('testCenter.stop') }}</button>
                <button v-else type="submit" class="btn btn-primary" :disabled="!canSend || !imageAccess || !imagePrompt.trim()" data-test="test-image-send"><Icon name="play" size="sm" />{{ t('testCenter.generate') }}</button>
              </div>
            </form>
            <div v-if="imageError" role="alert" class="test-error">{{ imageError }}<p v-if="imageRequestId" class="mt-1 break-all text-[10px]">{{ t('testCenter.requestId') }}: {{ imageRequestId }}</p></div>
            <p v-if="imageStopped" class="text-xs text-amber-700 dark:text-amber-300">{{ t('testCenter.stopped') }}</p>
            <div v-if="!imageResult && !busy" class="flex min-h-[260px] flex-col items-center justify-center rounded-2xl border border-dashed border-gray-200 p-6 text-center dark:border-dark-600">
              <Icon name="sparkles" size="xl" class="mb-3 text-primary-400" />
              <h2 class="font-bold text-gray-900 dark:text-white">{{ t('testCenter.imageWelcome') }}</h2>
              <p class="mt-2 max-w-sm text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('testCenter.imageWelcomeHint') }}</p>
            </div>
            <div v-if="busy" class="flex min-h-[260px] flex-col items-center justify-center gap-3 rounded-2xl bg-gray-50 text-sm text-gray-500 dark:bg-dark-700/70 dark:text-gray-400" role="status"><Icon name="refresh" size="lg" class="animate-spin" />{{ t('testCenter.imageWaiting') }}</div>
            <div v-if="imageResult" class="space-y-3" data-test="test-image-result">
              <p class="break-words text-sm font-semibold text-gray-800 dark:text-gray-100">{{ imageResultModel }} <span class="ml-2 text-xs font-normal text-gray-500">{{ t('testCenter.elapsed', { seconds: seconds(imageResult.durationMs) }) }}</span></p>
              <figure v-for="(image, index) in imageResult.images" :key="index" class="space-y-3">
                <img :src="image.src" :alt="imageResultPrompt" referrerpolicy="no-referrer" class="mx-auto max-h-[600px] max-w-full rounded-2xl border border-gray-100 object-contain dark:border-dark-600" @error="imagePreviewError = true" />
                <p v-if="imagePreviewError" class="text-xs text-amber-700 dark:text-amber-300">{{ t('testCenter.imagePreviewFailed') }}</p>
                <figcaption v-if="image.revisedPrompt" class="whitespace-pre-wrap break-words text-xs leading-5 text-gray-500 dark:text-gray-400">{{ image.revisedPrompt }}</figcaption>
                <a :href="image.src" :download="`test-image-${index + 1}.png`" target="_blank" rel="noopener noreferrer" class="btn btn-secondary btn-sm">{{ t('testCenter.saveImage') }}</a>
              </figure>
              <p class="break-all text-[10px] text-gray-400">{{ t('testCenter.requestId') }}: {{ imageResult.requestId }}</p>
            </div>
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { keysAPI } from '@/api/keys'
import {
  isTestChatModel, isTestImageModel, keyUnavailableReason, listTestModels,
  runChatTest, runImageTest, supportsTestImages, TestCenterError,
  type ChatMessage, type ChatTestResult, type ImageTestResult,
} from '@/api/testCenter'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { ApiKey } from '@/types'

interface DisplayMessage extends ChatMessage {
  id: number
  model?: string
  status?: 'pending' | 'success' | 'error' | 'stopped'
  error?: string
  requestId?: string
  result?: ChatTestResult
}

const { t } = useI18n()
const keys = ref<ApiKey[]>([])
const selectedKeyId = ref('')
const selectedKey = computed(() => keys.value.find(key => String(key.id) === selectedKeyId.value) ?? null)
const usableKeys = computed(() => keys.value.filter(key => !keyUnavailableReason(key)))
const loadingKeys = ref(false)
const keysError = ref('')
const loadingModels = ref(false)
const modelsError = ref('')
const models = ref<string[]>([])
const chatModel = ref('')
const imageModel = ref('')
const tab = ref<'chat' | 'image'>('chat')
const chatModels = computed(() => models.value.filter(isTestChatModel))
const imageModels = computed(() => models.value.filter(isTestImageModel))
const modelOptions = computed(() => tab.value === 'chat' ? chatModels.value : imageModels.value)
const selectedModel = computed({ get: () => tab.value === 'chat' ? chatModel.value : imageModel.value, set: value => { if (tab.value === 'chat') chatModel.value = value; else imageModel.value = value } })
const imageAccess = computed(() => !!selectedKey.value && supportsTestImages(selectedKey.value))
const busy = ref(false)
const canSend = computed(() => !!selectedKey.value && !keyUnavailableReason(selectedKey.value) && !!selectedModel.value && !loadingKeys.value && !loadingModels.value && !modelsError.value && !busy.value)
const chatPrompt = ref('')
const messages = ref<DisplayMessage[]>([])
let history: ChatMessage[] = []
let nextMessageId = 0
const transcript = ref<HTMLElement | null>(null)
const imagePrompt = ref('')
const imageResult = ref<ImageTestResult | null>(null)
const imageResultModel = ref('')
const imageResultPrompt = ref('')
const imageError = ref('')
const imageRequestId = ref('')
const imageStopped = ref(false)
const imagePreviewError = ref(false)
let keysController: AbortController | undefined
let modelsController: AbortController | undefined
let requestController: AbortController | undefined
let modelSequence = 0

const seconds = (milliseconds: number) => (milliseconds / 1000).toFixed(2)
function errorMessage(error: unknown): string {
  if (error instanceof TestCenterError) return error.message || t(`testCenter.errors.${error.code}`)
  return extractApiErrorMessage(error, t('testCenter.errors.requestFailed'))
}
function clearChat() { messages.value = []; history = [] }
function clearImages() {
  imageResult.value = null; imageError.value = ''; imageRequestId.value = ''; imageStopped.value = false; imagePreviewError.value = false
}
function stopRequest() { requestController?.abort() }
function dispose() { keysController?.abort(); modelsController?.abort(); stopRequest() }

async function loadKeys() {
  keysController?.abort()
  const controller = new AbortController()
  keysController = controller
  loadingKeys.value = true
  keysError.value = ''
  try {
    const result: ApiKey[] = []
    for (let page = 1; ; page++) {
      const response = await keysAPI.list(page, 100, undefined, { signal: controller.signal })
      result.push(...response.items)
      if (result.length >= response.total || !response.items.length) break
    }
    if (controller.signal.aborted) return
    keys.value = result
    const previousId = selectedKeyId.value
    if (!selectedKey.value || keyUnavailableReason(selectedKey.value)) selectedKeyId.value = String(usableKeys.value[0]?.id ?? '')
    if (previousId === selectedKeyId.value) await loadModels()
  } catch (error) {
    if (!controller.signal.aborted) keysError.value = extractApiErrorMessage(error, t('testCenter.errors.loadKeys'))
  } finally {
    if (!controller.signal.aborted) loadingKeys.value = false
  }
}

async function loadModels() {
  modelsController?.abort()
  const sequence = ++modelSequence
  const controller = new AbortController()
  modelsController = controller
  models.value = []; modelsError.value = ''; chatModel.value = ''; imageModel.value = ''
  loadingModels.value = false
  const key = selectedKey.value
  if (!key || keyUnavailableReason(key)) return
  loadingModels.value = true
  try {
    const result = await listTestModels({ apiKey: key.key, signal: controller.signal })
    if (sequence !== modelSequence || controller.signal.aborted) return
    models.value = result
    chatModel.value = result.find(model => model === 'gpt-6.1-sol') ?? chatModels.value[0] ?? ''
    imageModel.value = imageModels.value[0] ?? ''
  } catch (error) {
    if (sequence === modelSequence && !controller.signal.aborted) modelsError.value = errorMessage(error)
  } finally {
    if (sequence === modelSequence) loadingModels.value = false
  }
}

watch(selectedKeyId, () => { stopRequest(); clearChat(); clearImages(); void loadModels() })
watch(chatModel, clearChat)

async function scrollToReply() {
  await nextTick()
  if (transcript.value) transcript.value.scrollTop = transcript.value.scrollHeight
}

async function sendChat() {
  const key = selectedKey.value
  const prompt = chatPrompt.value.trim()
  if (tab.value !== 'chat' || !canSend.value || !key || !prompt) return
  const model = chatModel.value
  const user: ChatMessage = { role: 'user', content: prompt }
  messages.value.push({ ...user, id: ++nextMessageId })
  const reply = ref<DisplayMessage>({ id: ++nextMessageId, role: 'assistant', model, content: '', status: 'pending' })
  messages.value.push(reply.value)
  chatPrompt.value = ''
  busy.value = true
  const controller = new AbortController()
  requestController = controller
  void scrollToReply()
  try {
    const result = await runChatTest({ apiKey: key.key, model, messages: [...history, user], signal: controller.signal, onContent: content => { reply.value.content = content; void scrollToReply() } })
    reply.value.content = result.content
    reply.value.status = 'success'
    reply.value.result = result
    reply.value.requestId = result.requestId
    history.push(user, { role: 'assistant', content: result.content })
  } catch (error) {
    reply.value.status = controller.signal.aborted ? 'stopped' : 'error'
    if (!controller.signal.aborted) {
      reply.value.error = errorMessage(error)
      if (error instanceof TestCenterError) reply.value.requestId = error.requestId
    }
  } finally {
    busy.value = false
    requestController = undefined
    void scrollToReply()
  }
}

function onPromptKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter' || event.shiftKey || event.isComposing || event.keyCode === 229) return
  event.preventDefault()
  void sendChat()
}

async function generateImage() {
  const key = selectedKey.value
  const prompt = imagePrompt.value.trim()
  if (tab.value !== 'image' || !canSend.value || !key || !imageAccess.value || !prompt) return
  clearImages()
  busy.value = true
  const controller = new AbortController()
  requestController = controller
  try {
    const model = imageModel.value
    const result = await runImageTest({ apiKey: key.key, model, prompt, signal: controller.signal })
    if (controller.signal.aborted) return
    imageResult.value = result
    imageResultModel.value = model
    imageResultPrompt.value = prompt
  } catch (error) {
    imageStopped.value = controller.signal.aborted
    if (!controller.signal.aborted) {
      imageError.value = errorMessage(error)
      if (error instanceof TestCenterError) imageRequestId.value = error.requestId
    }
  } finally {
    busy.value = false
    requestController = undefined
  }
}

onMounted(loadKeys)
onBeforeUnmount(dispose)
onBeforeRouteLeave(dispose)
</script>

<style scoped>
.test-error { @apply rounded-xl bg-red-50 px-3 py-2 text-xs leading-5 text-red-700 dark:bg-red-900/20 dark:text-red-300; overflow-wrap: anywhere; }
</style>
