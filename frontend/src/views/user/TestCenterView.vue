<template>
  <AppLayout :header-title="t('testCenter.title')" :header-subtitle="t('testCenter.subtitle')" header-icon="bot" fill>
    <div class="studio-shell" data-testid="inspiration-studio">

      <div v-if="keysError" role="alert" class="test-error">{{ keysError }}</div>
      <div v-if="modelsError" role="alert" class="test-error">{{ modelsError }} <button type="button" class="font-semibold underline" :disabled="busy" @click="loadModels">{{ t('testCenter.retry') }}</button></div>

      <div class="studio-workspace">
        <div class="studio-modebar">
          <div class="studio-tabs" role="tablist" :aria-label="t('testCenter.tabsLabel')">
            <button id="test-chat-tab" type="button" role="tab" aria-controls="test-chat-panel" :aria-selected="tab === 'chat'" :disabled="busy" @click="tab = 'chat'"><WorkspaceIcon name="bot" /><span>{{ t('testCenter.chatTab') }}</span></button>
            <button id="test-image-tab" type="button" role="tab" aria-controls="test-image-panel" :aria-selected="tab === 'image'" :disabled="busy" @click="tab = 'image'"><WorkspaceIcon name="image" /><span>{{ t('testCenter.imageTab') }}</span></button>
          </div>
          <div class="studio-header-actions">
          <RouterLink to="/keys" class="studio-text-action" :aria-label="t('testCenter.manageKeys')"><Icon name="key" size="sm" /><span>{{ t('testCenter.manageKeys') }}</span></RouterLink>
          <button type="button" class="studio-icon-action" :title="t('testCenter.refresh')" :aria-label="t('testCenter.refresh')" :disabled="loadingKeys || loadingModels || busy" @click="loadKeys"><Icon name="refresh" size="sm" :class="loadingKeys || loadingModels ? 'animate-spin' : ''" /></button>
          <button type="button" class="studio-clear" :disabled="busy" @click="tab === 'chat' ? clearChat() : clearImages()"><Icon name="trash" size="sm" /><span>{{ tab === 'chat' ? t('testCenter.clearChat') : t('testCenter.clearImages') }}</span></button>
          </div>
        </div>

        <div v-if="tab === 'chat'" id="test-chat-panel" class="studio-stage" role="tabpanel" aria-labelledby="test-chat-tab">
          <div ref="transcript" class="studio-transcript" role="log" :aria-label="t('testCenter.conversation')" aria-live="polite" aria-relevant="additions text" data-test="test-conversation">
            <div v-if="!messages.length" class="studio-welcome">
              <div class="studio-mascot"><span class="studio-orbit" /><StudioBot /><span class="studio-mascot-dot" /></div>
              <span class="studio-eyebrow">{{ t('testCenter.welcomeEyebrow') }}</span>
              <h2>{{ t('testCenter.chatWelcome') }}</h2>
              <p>{{ t('testCenter.chatWelcomeHint') }}</p>
              <div class="studio-starters">
                <button v-for="idea in chatIdeas" :key="idea.id" type="button" @click="useIdea(idea.prompt)"><span class="studio-idea-icon" :class="`idea-${idea.id}`"><Icon :name="idea.icon" size="sm" /></span><span><strong>{{ t(idea.title) }}</strong><small>{{ t(idea.hint) }}</small></span><Icon name="arrowRight" size="sm" class="studio-idea-arrow" /></button>
              </div>
            </div>
            <div v-else class="studio-messages">
              <article v-for="message in messages" :key="message.id" class="studio-message" :class="{ 'studio-message-user': message.role === 'user' }" :data-test="`test-message-${message.role}`">
                <span v-if="message.role === 'assistant'" class="studio-message-avatar"><WorkspaceIcon name="bot" /></span>
                <div class="studio-message-body">
                  <div class="studio-message-name">{{ message.role === 'user' ? t('testCenter.you') : message.model }}<span v-if="message.status === 'pending'" class="studio-typing"><i /><i /><i /></span></div>
                  <StudioReply v-if="message.status === 'success'" :content="message.content" />
                  <div v-else class="studio-message-text">{{ message.content || (message.status === 'pending' ? t('testCenter.waitingReply') : '') }}</div>
                  <div v-if="message.error" role="alert" class="mt-2 break-words text-xs text-red-600 dark:text-red-300">{{ message.error }}</div>
                  <p v-if="message.status === 'stopped'" class="studio-message-note">{{ t('testCenter.stopped') }}</p>
                  <p v-if="message.result?.finishReason === 'length'" class="studio-message-note">{{ t('testCenter.lengthLimit') }}</p>
                  <div v-if="message.result" class="studio-message-metrics"><span><Icon name="clock" size="xs" />{{ t('testCenter.elapsed', { seconds: seconds(message.result.durationMs) }) }}</span><span v-if="message.result.firstTokenMs !== undefined">{{ t('testCenter.firstToken', { seconds: seconds(message.result.firstTokenMs) }) }}</span><span v-if="message.result.usage">{{ t('testCenter.tokens', { input: message.result.usage.inputTokens, output: message.result.usage.outputTokens }) }}</span></div>
                  <details v-if="message.requestId" class="studio-request-id"><summary>{{ t('testCenter.requestId') }}</summary>{{ message.requestId }}</details>
                </div>
              </article>
            </div>
          </div>
        </div>

        <div v-else id="test-image-panel" class="studio-stage studio-image-stage" role="tabpanel" aria-labelledby="test-image-tab">
          <div v-if="busy" class="studio-welcome" role="status"><div class="studio-mascot"><StudioBot active /></div><h2>{{ t('testCenter.imageWaiting') }}</h2><p>{{ t('testCenter.imageWaitingHint') }}</p></div>
          <div v-else-if="imageResult" class="studio-image-result" data-test="test-image-result">
            <div class="studio-image-result-heading"><span class="studio-mini-bot"><WorkspaceIcon name="image" /></span><div><strong>{{ imageResultModel }}</strong><small>{{ t('testCenter.elapsed', { seconds: seconds(imageResult.durationMs) }) }}</small></div></div>
            <div v-for="(image, index) in imageResult.images" :key="index" class="studio-generated-image"><img :src="image.src" :alt="imageResultPrompt" referrerpolicy="no-referrer" @error="imagePreviewError = true" /><p v-if="imagePreviewError" class="studio-message-note">{{ t('testCenter.imagePreviewFailed') }}</p><p v-if="image.revisedPrompt" class="studio-image-caption">{{ image.revisedPrompt }}</p><a :href="image.src" :download="`test-image-${index + 1}.png`" target="_blank" rel="noopener noreferrer" class="studio-save"><Icon name="download" size="sm" />{{ t('testCenter.saveImage') }}</a></div>
            <details v-if="imageResult.requestId" class="studio-request-id"><summary>{{ t('testCenter.requestId') }}</summary>{{ imageResult.requestId }}</details>
          </div>
          <div v-else class="studio-welcome"><div class="studio-mascot studio-mascot-image"><span class="studio-orbit" /><StudioBot /><span class="studio-mascot-dot" /></div><span class="studio-eyebrow">{{ t('testCenter.imageEyebrow') }}</span><h2>{{ t('testCenter.imageWelcome') }}</h2><p>{{ t('testCenter.imageWelcomeHint') }}</p><div class="studio-starters"><button v-for="idea in imageIdeas" :key="idea.id" type="button" @click="useIdea(idea.prompt)"><span class="studio-idea-icon" :class="`idea-${idea.id}`"><Icon :name="idea.icon" size="sm" /></span><span><strong>{{ t(idea.title) }}</strong><small>{{ t(idea.hint) }}</small></span><Icon name="arrowRight" size="sm" class="studio-idea-arrow" /></button></div></div>
          <div v-if="imageError" role="alert" class="test-error">{{ imageError }}<details v-if="imageRequestId" class="studio-request-id"><summary>{{ t('testCenter.requestId') }}</summary>{{ imageRequestId }}</details></div>
          <p v-if="imageStopped" class="studio-message-note">{{ t('testCenter.stopped') }}</p>
        </div>

        <div class="studio-compose-area">
          <p v-if="!loadingKeys && !keysError && !usableKeys.length" class="studio-notice">{{ t('testCenter.noKeys') }} <RouterLink to="/keys">{{ t('testCenter.manageKeys') }}</RouterLink></p>
          <p v-if="selectedKey && tab === 'chat' && !loadingModels && !modelsError && !chatModels.length" class="studio-notice">{{ t('testCenter.noChatModels') }}</p>
          <p v-if="selectedKey && tab === 'image' && !imageAccess" class="studio-notice" data-test="test-image-permission">{{ t('testCenter.imageNotAllowed') }}</p>
          <p v-else-if="selectedKey && tab === 'image' && !loadingModels && !modelsError && !imageModels.length" class="studio-notice">{{ t('testCenter.noImageModels') }}</p>
          <form class="studio-composer" @submit.prevent="tab === 'chat' ? sendChat() : generateImage()">
            <label :for="tab === 'chat' ? 'test-chat-prompt' : 'test-image-prompt'" class="sr-only">{{ t(tab === 'chat' ? 'testCenter.chatPrompt' : 'testCenter.imagePrompt') }}</label>
            <textarea v-if="tab === 'chat'" id="test-chat-prompt" ref="chatInput" v-model="chatPrompt" rows="3" maxlength="16000" :placeholder="t('testCenter.chatPlaceholder')" :disabled="busy" data-test="test-chat-prompt" @keydown="onPromptKeydown" />
            <textarea v-else id="test-image-prompt" ref="imageInput" v-model="imagePrompt" rows="3" maxlength="16000" :placeholder="t('testCenter.imagePlaceholder')" :disabled="busy" data-test="test-image-prompt" />
            <div class="studio-composer-toolbar">
              <div class="studio-select-field studio-key-field"><Icon name="key" size="sm" /><Select id="test-key" v-model="selectedKeyId" :options="keyOptions" :disabled="loadingKeys || busy" :placeholder="t(loadingKeys ? 'testCenter.loadingKeys' : 'testCenter.selectKey')" :aria-label="t('testCenter.apiKey')" searchable data-test="test-key" /></div>
              <div class="studio-select-field studio-model-field"><WorkspaceIcon name="models" /><Select id="test-model" v-model="selectedModel" :options="modelOptions.map(model => ({ value: model, label: model }))" :disabled="!selectedKey || loadingModels || busy || !modelOptions.length" :placeholder="t(loadingModels ? 'testCenter.loadingModels' : 'testCenter.selectModel')" :aria-label="t('testCenter.model')" searchable data-test="test-model" /></div>
              <button v-if="busy" type="button" class="studio-send studio-stop" @click="stopRequest"><span class="studio-stop-square" />{{ t('testCenter.stop') }}</button>
              <button v-else-if="tab === 'chat'" type="submit" class="studio-send" :disabled="!canSend || !chatPrompt.trim()" data-test="test-chat-send"><span>{{ t('testCenter.send') }}</span><Icon name="arrowUp" size="sm" /></button>
              <button v-else type="submit" class="studio-send" :disabled="!canSend || !imageAccess || !imagePrompt.trim()" data-test="test-image-send"><span>{{ t('testCenter.generate') }}</span><Icon name="sparkles" size="sm" /></button>
            </div>
          </form>
        </div>
        <div class="studio-footnote"><span class="studio-session-dot" :class="{ ready: canSend }" /><span>{{ t('testCenter.billingShort') }}</span><RouterLink to="/usage">{{ t('testCenter.viewUsage') }}<Icon name="arrowRight" size="xs" /></RouterLink></div>
          <details class="studio-session-info" data-test="test-key-info"><summary>{{ t('testCenter.sessionInfo') }}<Icon name="chevronDown" size="xs" /><span class="studio-key-summary">{{ selectedKey?.group?.name }}<template v-if="routeCount > 1"> +{{ routeCount - 1 }}</template></span></summary><div class="studio-session-content"><div v-if="selectedKey" class="studio-key-facts"><span>{{ t('testCenter.group') }}<strong>{{ selectedKey.group?.name }}<template v-if="routeCount > 1"> · {{ t('testCenter.routeCount', { count: routeCount }) }}</template></strong></span><span>{{ t('testCenter.quota') }}<strong>{{ selectedKey.quota > 0 ? `$${Math.max(0, selectedKey.quota - selectedKey.quota_used).toFixed(2)}` : t('testCenter.unlimited') }}</strong></span></div><p>{{ t('testCenter.billingHint') }}</p><p>{{ t('testCenter.sessionHint') }}</p><p>{{ t(tab === 'chat' ? 'testCenter.sendHint' : 'testCenter.oneImageHint') }}</p></div></details>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import { userGroupsAPI } from '@/api/groups'
import Icon from '@/components/icons/Icon.vue'
import WorkspaceIcon from '@/components/workspace/WorkspaceIcon.vue'
import StudioBot from '@/components/studio/StudioBot.vue'
import StudioReply from '@/components/studio/StudioReply.vue'
import { keysAPI } from '@/api/keys'
import {
  isTestChatModel, isTestImageModel, keyUnavailableReason, listTestModels,
  runChatTest, runImageTest, supportsTestImages, TestCenterError,
  type ChatMessage, type ChatTestResult, type ImageTestResult,
} from '@/api/testCenter'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { ApiKey, Group } from '@/types'

interface DisplayMessage extends ChatMessage {
  id: number
  model?: string
  status?: 'pending' | 'success' | 'error' | 'stopped'
  error?: string
  requestId?: string
  result?: ChatTestResult
}

const { t } = useI18n()
const chatInput = ref<HTMLTextAreaElement | null>(null)
const imageInput = ref<HTMLTextAreaElement | null>(null)
const chatIdeas = [
  { id: 'write', icon: 'document', title: 'testCenter.ideas.write.title', hint: 'testCenter.ideas.write.hint', prompt: 'testCenter.ideas.write.prompt' },
  { id: 'code', icon: 'terminal', title: 'testCenter.ideas.code.title', hint: 'testCenter.ideas.code.hint', prompt: 'testCenter.ideas.code.prompt' },
  { id: 'learn', icon: 'book', title: 'testCenter.ideas.learn.title', hint: 'testCenter.ideas.learn.hint', prompt: 'testCenter.ideas.learn.prompt' },
  { id: 'plan', icon: 'sparkles', title: 'testCenter.ideas.plan.title', hint: 'testCenter.ideas.plan.hint', prompt: 'testCenter.ideas.plan.prompt' },
] as const
const imageIdeas = [
  { id: 'write', icon: 'sun', title: 'testCenter.imageIdeas.landscape.title', hint: 'testCenter.imageIdeas.landscape.hint', prompt: 'testCenter.imageIdeas.landscape.prompt' },
  { id: 'code', icon: 'sparkles', title: 'testCenter.imageIdeas.robot.title', hint: 'testCenter.imageIdeas.robot.hint', prompt: 'testCenter.imageIdeas.robot.prompt' },
  { id: 'learn', icon: 'grid', title: 'testCenter.imageIdeas.poster.title', hint: 'testCenter.imageIdeas.poster.hint', prompt: 'testCenter.imageIdeas.poster.prompt' },
  { id: 'plan', icon: 'cube', title: 'testCenter.imageIdeas.world.title', hint: 'testCenter.imageIdeas.world.hint', prompt: 'testCenter.imageIdeas.world.prompt' },
] as const
function useIdea(promptKey: string) {
  if (tab.value === 'chat') chatPrompt.value = t(promptKey)
  else imagePrompt.value = t(promptKey)
  void nextTick(() => (tab.value === 'chat' ? chatInput.value : imageInput.value)?.focus({ preventScroll: true }))
}
const keys = ref<ApiKey[]>([])
const groups = ref<Group[]>([])
const keyReason = (key: ApiKey) => keyUnavailableReason(key, Date.now(), groups.value)
const keyOptions = computed(() => keys.value.map(key => ({ value: String(key.id), label: `${key.name} · #${key.id}${keyReason(key) ? ` · ${t(`testCenter.${keyReason(key)}`)}` : ''}`, disabled: !!keyReason(key) })))
const selectedKeyId = ref('')
const selectedKey = computed(() => keys.value.find(key => String(key.id) === selectedKeyId.value) ?? null)
const routeCount = computed(() => selectedKey.value?.group_ids?.length || (selectedKey.value?.group_id ? 1 : 0))
const usableKeys = computed(() => keys.value.filter(key => !keyReason(key)))
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
const imageAccess = computed(() => !!selectedKey.value && supportsTestImages(selectedKey.value, groups.value))
const busy = ref(false)
const canSend = computed(() => !!selectedKey.value && !keyReason(selectedKey.value) && !!selectedModel.value && !loadingKeys.value && !loadingModels.value && !modelsError.value && !busy.value)
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
    const [availableGroups, firstPage] = await Promise.all([userGroupsAPI.getAvailable({ signal: controller.signal }), keysAPI.list(1, 100, undefined, { signal: controller.signal })])
    const result: ApiKey[] = []
    for (let page = 1; ; page++) {
      const response = page === 1 ? firstPage : await keysAPI.list(page, 100, undefined, { signal: controller.signal })
      result.push(...response.items)
      if (result.length >= response.total || !response.items.length) break
    }
    if (controller.signal.aborted) return
    groups.value = availableGroups
    keys.value = result
    const previousId = selectedKeyId.value
    if (!selectedKey.value || keyReason(selectedKey.value)) selectedKeyId.value = String(usableKeys.value[0]?.id ?? '')
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
  if (!key || keyReason(key)) return
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
.studio-shell { --studio-accent: #219d8b; --studio-ink: #273b50; --studio-muted: #8193a6; --studio-line: #dfe9ef; --studio-surface: #fff; --studio-soft: #f4f8fb; display: flex; flex-direction: column; gap: 16px; height: 100%; min-height: 0; width: 100%; max-width: none; margin: 0 auto; color: var(--studio-ink); }
.studio-header,.studio-heading,.studio-header-actions { display: flex; align-items: center; gap: 12px; min-width: 0; }
.studio-header { justify-content: space-between; padding: 0 3px; flex-shrink: 0; }
.studio-heading h1 { font-size: 21px; font-weight: 750; letter-spacing: -.5px; white-space: nowrap; }
.studio-mini-bot { width: 36px; height: 36px; flex-shrink: 0; display: inline-flex; align-items: center; justify-content: center; border-radius: 12px; color: var(--studio-accent); background: rgb(51 174 151 / .09); }
.studio-mini-bot svg { width: 21px; height: 21px; }
.studio-edition { border-left: 1px solid var(--studio-line); padding-left: 12px; font-size: 12px; color: var(--studio-muted); }
.studio-text-action,.studio-icon-action,.studio-clear { display: inline-flex; align-items: center; justify-content: center; gap: 7px; color: var(--studio-muted); font-size: 12px; transition: color .2s, background .2s; }
.studio-text-action:hover,.studio-icon-action:hover,.studio-clear:hover { color: var(--studio-accent); }
.studio-icon-action { width: 34px; height: 34px; border: 1px solid var(--studio-line); border-radius: 11px; background: var(--studio-surface); }
.studio-shell button:disabled { opacity: .45; cursor: not-allowed; }
.studio-shell button:focus-visible,.studio-shell a:focus-visible,.studio-session-info summary:focus-visible { outline: 2px solid var(--studio-accent); outline-offset: 4px; }
.studio-workspace { flex: 1; min-height: 0; display: flex; flex-direction: column; padding: 16px 24px 12px; border-radius: 28px; border: 1px solid rgb(255 255 255 / .95); background: radial-gradient(ellipse at 84% 16%, rgb(206 233 236 / .28), transparent 45%), radial-gradient(ellipse at 10% 80%, rgb(217 225 248 / .28), transparent 50%), rgb(249 252 255 / .73); box-shadow: 0 12px 36px rgb(40 76 102 / .035); }
.studio-modebar { display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-shrink: 0; }
.studio-tabs { display: inline-flex; gap: 3px; padding: 4px; border: 1px solid var(--studio-line); border-radius: 14px; background: var(--studio-soft); }
.studio-tabs button { display: inline-flex; align-items: center; justify-content: center; gap: 7px; min-width: 92px; padding: 8px 13px; border-radius: 10px; font-size: 12px; font-weight: 600; color: var(--studio-muted); transition: background .2s,color .2s,box-shadow .2s; }
.studio-tabs svg { width: 16px; height: 16px; }
.studio-tabs button[aria-selected="true"] { color: var(--studio-accent); background: var(--studio-surface); box-shadow: 0 2px 6px rgb(30 67 88 / .06); }
.studio-clear { padding: 10px 4px; }
.studio-stage { flex: 1; min-height: 0; display: flex; flex-direction: column; }
.studio-transcript { flex: 1; min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: 12px 8px; scrollbar-width: thin; scrollbar-color: rgb(124 144 164 / .22) transparent; }
.studio-welcome { min-height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; padding: 22px 0 28px; text-align: center; }
.studio-mascot { position: relative; width: 106px; height: 106px; display: grid; place-items: center; margin-bottom: -2px; color: var(--studio-accent); }
.studio-mascot > svg { position: relative; width: 100px; height: 100px; z-index: 1; }
.studio-orbit { position: absolute; inset: 4px; border: 1px solid rgb(51 174 151 / .14); border-radius: 36px; background: linear-gradient(135deg,rgb(255 255 255 / .8),rgb(214 242 234 / .36)); transform: rotate(-12deg); }
.studio-mascot-dot { position: absolute; left: -2px; bottom: 19px; width: 7px; height: 7px; background: #b8c9f1; border-radius: 50%; box-shadow: 98px -70px 0 -1px #eacb91; }
.studio-eyebrow { font-size: 10px; font-weight: 700; letter-spacing: 2.4px; color: var(--studio-accent); }
.studio-welcome h2 { font-size: clamp(23px,2.2vw,30px); line-height: 1.35; letter-spacing: -.8px; font-weight: 750; }
.studio-welcome > p { max-width: 460px; font-size: 13px; line-height: 1.8; color: var(--studio-muted); }
.studio-starters { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 10px; width: 100%; max-width: 600px; margin-top: 12px; text-align: left; }
.studio-starters button { display: flex; align-items: center; gap: 12px; min-width: 0; padding: 13px 15px; border: 1px solid var(--studio-line); border-radius: 15px; background: rgb(255 255 255 / .62); transition: transform .2s,border-color .2s,background .2s,box-shadow .2s; }
.studio-starters button:hover { transform: translateY(-2px); background: var(--studio-surface); border-color: rgb(51 174 151 / .35); box-shadow: 0 5px 16px rgb(30 100 90 / .05); }
.studio-starters button > span:nth-child(2) { flex: 1; min-width: 0; }
.studio-starters strong { display: block; margin-bottom: 3px; font-size: 12px; font-weight: 650; }
.studio-starters small { display: block; font-size: 11px; color: var(--studio-muted); line-height: 1.5; }
.studio-idea-icon { width: 33px; height: 33px; display: grid; place-items: center; flex-shrink: 0; border-radius: 10px; }
.idea-write { color: #459f91; background: rgb(69 159 145 / .09); }.idea-code { color: #7a91d0; background: rgb(122 145 208 / .1); }.idea-learn { color: #bc945d; background: rgb(188 148 93 / .1); }.idea-plan { color: #ac88b8; background: rgb(172 136 184 / .1); }
.studio-idea-arrow { color: var(--studio-muted); opacity: .45; flex-shrink: 0; }
.studio-compose-area { flex-shrink: 0; width: 100%; max-width: 920px; margin: 0 auto; }
.studio-composer { border: 1px solid var(--studio-line); border-radius: 21px; background: var(--studio-surface); box-shadow: 0 6px 24px rgb(44 81 106 / .045); transition: border-color .2s,box-shadow .2s; }
.studio-composer:focus-within { border-color: rgb(51 174 151 / .48); box-shadow: 0 0 0 3px rgb(51 174 151 / .05),0 8px 24px rgb(44 81 106 / .045); }
.studio-composer textarea { display: block; width: 100%; min-height: 85px; max-height: 180px; resize: vertical; border: 0; background: transparent; padding: 18px 20px 10px; font-size: 14px; line-height: 1.7; color: var(--studio-ink); border-radius: 21px 21px 0 0; box-shadow: none; outline: none; }
.studio-composer textarea::placeholder { color: var(--studio-muted); opacity: .85; }
.studio-composer-toolbar { display: flex; align-items: center; gap: 8px; padding: 9px 12px 12px; }
.studio-select-field { display: flex; align-items: center; gap: 7px; min-width: 0; height: 36px; padding: 0 10px; border: 1px solid var(--studio-line); border-radius: 11px; background: var(--studio-soft); color: var(--studio-muted); }
.studio-select-field > svg { flex-shrink: 0; width: 15px; height: 15px; pointer-events: none; }
.studio-select-field select { min-width: 0; width: 100%; appearance: none; border: 0; outline: none; background: transparent; box-shadow: none; padding: 0; font-size: 12px; font-weight: 550; color: var(--studio-ink); text-overflow: ellipsis; cursor: pointer; }
.studio-select-field:focus-within { outline: 2px solid rgb(51 174 151 / .4); outline-offset: 1px; }
.studio-select-field select option { color: var(--studio-ink); background: var(--studio-surface); }
.studio-key-field { flex: 0 1 230px; }.studio-model-field { flex: 0 1 260px; }
.studio-send { display: inline-flex; align-items: center; justify-content: center; gap: 8px; margin-left: auto; flex-shrink: 0; min-height: 36px; padding: 0 17px; border-radius: 11px; background: var(--studio-accent); color: white; font-size: 12px; font-weight: 650; box-shadow: 0 3px 8px rgb(33 157 139 / .12); transition: filter .2s,transform .2s; }
.studio-send:not(:disabled):hover { filter: brightness(1.06); transform: translateY(-1px); }.studio-stop { background: var(--studio-soft); color: var(--studio-ink); border: 1px solid var(--studio-line); box-shadow: none; }.studio-stop-square { width: 9px; height: 9px; border-radius: 2px; background: var(--studio-ink); }
.studio-footnote { display: flex; align-items: center; justify-content: flex-start; gap: 6px; flex-wrap: wrap; padding: 9px 0 5px; font-size: 10px; color: var(--studio-muted); }
.studio-footnote a { display: inline-flex; align-items: center; gap: 3px; margin-left: 5px; color: var(--studio-accent); }.studio-session-dot { width: 5px; height: 5px; border-radius: 50%; background: #a8b6c4; }.studio-session-dot.ready { background: var(--studio-accent); }
.studio-session-info { color: var(--studio-muted); font-size: 11px; }.studio-session-info summary { display: flex; align-items: center; gap: 5px; width: fit-content; max-width: 100%; margin: 0; cursor: pointer; list-style: none; }.studio-session-info summary::-webkit-details-marker { display: none; }.studio-session-info[open] summary > svg { transform: rotate(180deg); }.studio-key-summary { max-width: 260px; margin-left: 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; opacity: .7; }
.studio-session-content { margin-top: 10px; padding: 12px 16px; border-radius: 13px; background: var(--studio-soft); border: 1px solid var(--studio-line); line-height: 1.8; max-height: 180px; overflow: auto; }.studio-session-content p + p { margin-top: 5px; }.studio-key-facts { display: flex; flex-wrap: wrap; gap: 10px 28px; margin-bottom: 8px; }.studio-key-facts strong { display: inline; margin-left: 8px; color: var(--studio-ink); font-weight: 600; overflow-wrap: anywhere; }
.studio-notice { margin: 0 0 8px; padding: 8px 12px; background: rgb(206 166 91 / .09); color: #9a753d; border-radius: 11px; font-size: 12px; line-height: 1.7; }.studio-notice a { color: var(--studio-accent); }
.studio-messages { max-width: 1120px; margin: 0 auto; padding: 14px 4px 22px; }.studio-message { display: flex; align-items: flex-start; gap: 12px; margin-bottom: 28px; min-width: 0; }.studio-message-avatar { display: grid; place-items: center; flex-shrink: 0; width: 32px; height: 32px; border: 1px solid rgb(51 174 151 / .15); border-radius: 11px; background: rgb(51 174 151 / .07); color: var(--studio-accent); }.studio-message-avatar svg { width: 20px; height: 20px; }.studio-message-body { min-width: 0; flex: 1; }.studio-message-name { display: flex; align-items: center; gap: 10px; margin-bottom: 7px; font-size: 11px; font-weight: 650; color: var(--studio-muted); overflow-wrap: anywhere; }.studio-message-text { font-size: 14px; line-height: 1.85; white-space: pre-wrap; overflow-wrap: anywhere; }.studio-message-user { justify-content: flex-end; }.studio-message-user .studio-message-body { max-width: 85%; flex: initial; background: rgb(51 174 151 / .075); border: 1px solid rgb(51 174 151 / .09); border-radius: 18px 18px 5px 18px; padding: 12px 17px; }.studio-message-user .studio-message-name { color: var(--studio-accent); }
.studio-message-metrics { display: flex; flex-wrap: wrap; gap: 5px 14px; margin-top: 12px; color: var(--studio-muted); font-size: 10px; }.studio-message-metrics span { display: inline-flex; align-items: center; gap: 4px; }.studio-message-note { margin-top: 8px; font-size: 12px; line-height: 1.8; color: #ae874d; }.studio-request-id { margin-top: 8px; font-size: 10px; color: var(--studio-muted); overflow-wrap: anywhere; }.studio-request-id summary { cursor: pointer; opacity: .65; margin-bottom: 3px; }
.studio-typing { display: inline-flex; gap: 3px; }.studio-typing i { width: 4px; height: 4px; background: var(--studio-accent); border-radius: 50%; animation: studio-think 1.2s ease-in-out infinite; }.studio-typing i:nth-child(2) { animation-delay: .15s; }.studio-typing i:nth-child(3) { animation-delay: .3s; }@keyframes studio-think { 50% { opacity: .25; transform: translateY(-2px); } }
.studio-image-stage { overflow-y: auto; padding: 12px 8px; overscroll-behavior: contain; }.studio-image-stage .studio-welcome { flex: 1; }.studio-image-result { width: 100%; max-width: 700px; margin: 12px auto 24px; }.studio-image-result-heading { display: flex; align-items: center; gap: 10px; margin-bottom: 15px; }.studio-image-result-heading strong { display: block; font-size: 13px; }.studio-image-result-heading small { font-size: 11px; color: var(--studio-muted); }.studio-generated-image { padding: 10px; border-radius: 20px; background: var(--studio-surface); border: 1px solid var(--studio-line); }.studio-generated-image img { display: block; max-width: 100%; max-height: 540px; margin: 0 auto; border-radius: 13px; object-fit: contain; }.studio-image-caption { padding: 12px 5px; color: var(--studio-muted); font-size: 12px; overflow-wrap: anywhere; }.studio-save { display: inline-flex; align-items: center; gap: 6px; padding: 11px 8px 3px; font-size: 12px; color: var(--studio-accent); }
.test-error { @apply rounded-xl bg-red-50 px-3 py-2 text-xs leading-5 text-red-700 dark:bg-red-900/20 dark:text-red-300; overflow-wrap: anywhere; flex-shrink: 0; }
:global(.dark .studio-shell) { --studio-accent: #68c9b7; --studio-ink: #dbe5ef; --studio-muted: #8498ac; --studio-line: #2b3e50; --studio-surface: #1a2b3d; --studio-soft: #203245; }
:global(.dark .studio-workspace) { border-color: #2b3e50; background: radial-gradient(ellipse at 84% 16%,rgb(53 113 107 / .11),transparent 45%),radial-gradient(ellipse at 10% 80%,rgb(78 90 150 / .1),transparent 50%),#162536; }
:global(.dark .studio-starters button) { background: rgb(33 51 70 / .45); }:global(.dark .studio-starters button:hover) { background: #203547; }:global(.dark .studio-orbit) { background: linear-gradient(135deg,rgb(104 201 183 / .08),rgb(104 201 183 / .02)); }:global(.dark .studio-send:not(.studio-stop)) { background: #328e81; color: #f0fffc; }:global(.dark .studio-notice) { color: #c9a56e; }
@media (max-width: 767px) { .studio-shell { height: 100%; min-height: 0; gap: 12px; }.studio-edition { display: none; }.studio-heading { gap: 8px; }.studio-heading h1 { font-size: 19px; }.studio-header-actions { gap: 9px; }.studio-text-action span { display: none; }.studio-header .studio-text-action { width: 32px; height: 32px; }.studio-workspace { padding: 12px 12px 10px; border-radius: 22px; }.studio-tabs button { min-width: 72px; padding: 7px 9px; gap: 5px; }.studio-tabs { padding: 3px; }.studio-clear { font-size: 11px; gap: 4px; }.studio-welcome { padding: 20px 0; gap: 10px; }.studio-mascot { width: 82px; height: 82px; }.studio-mascot > svg { width: 84px; height: 84px; }.studio-mascot-dot { box-shadow: 78px -55px 0 -1px #eacb91; }.studio-welcome h2 { font-size: 23px; }.studio-welcome > p { max-width: 285px; font-size: 12px; }.studio-starters { gap: 7px; margin-top: 8px; }.studio-starters button { padding: 10px; gap: 8px; border-radius: 12px; }.studio-starters strong { font-size: 11px; }.studio-starters small { font-size: 10px; }.studio-idea-icon { width: 26px; height: 29px; border-radius: 9px; }.studio-idea-arrow { display: none; }.studio-composer { border-radius: 17px; }.studio-composer textarea { padding: 14px 14px 8px; min-height: 80px; font-size: 13px; }.studio-composer-toolbar { display: grid; grid-template-columns: minmax(0,1fr) auto; gap: 7px; padding: 7px 9px 9px; }.studio-key-field { grid-column: 1 / -1; }.studio-select-field { height: 34px; }.studio-send { height: 34px; min-height: 34px; padding: 0 12px; }.studio-footnote { font-size: 9px; gap: 4px; padding: 9px 0 6px; }.studio-session-info { font-size: 10px; }.studio-key-summary { max-width: 145px; }.studio-transcript,.studio-image-stage { padding-left: 0; padding-right: 0; }.studio-message { gap: 8px; margin-bottom: 22px; }.studio-message-avatar { width: 27px; height: 27px; border-radius: 9px; }.studio-message-avatar svg { width: 17px; height: 17px; }.studio-message-user .studio-message-body { max-width: 93%; padding: 10px 12px; }.studio-message-text { font-size: 13px; }.studio-session-content { padding: 10px 12px; }.studio-key-facts { flex-direction: column; gap: 4px; } }
@media (prefers-reduced-motion: reduce) { .studio-shell * { animation: none !important; transition: none !important; scroll-behavior: auto !important; } }

.studio-select-field { height: auto; padding: 0 0 0 10px; }
.studio-select-field > :deep(.relative) { flex: 1; min-width: 0; }
.studio-select-field :deep(.select-trigger) { min-height: 36px; border: 0; background: transparent; border-radius: 11px; box-shadow: none; font-size: 12px; padding-left: 0; }
.studio-header-actions { flex-wrap: wrap; justify-content: flex-end; }
.studio-compose-area { width: 100%; max-width: 1120px; }
@media (max-width: 767px) { .studio-workspace { padding: 10px; } .studio-modebar { align-items: flex-start; } .studio-header-actions { gap: 8px; } .studio-header-actions .studio-text-action span { display: none; } .studio-welcome { padding: 12px 0; } .studio-welcome .studio-starters { display: none; } .studio-welcome h2 { font-size: 21px; } .studio-composer textarea { min-height: 60px; } }
</style>
