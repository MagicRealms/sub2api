<template><div class="studio-reply" v-html="html"></div></template>
<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
const props = defineProps<{ content: string }>()
// Generated markup is text to discuss, never executable page content.
const escapeMarkup = (value: string) => value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
const html = computed(() => DOMPurify.sanitize(/^\s*</.test(props.content)
  ? `<pre><code>${escapeMarkup(props.content)}</code></pre>`
  : marked.parse(props.content, { async: false, gfm: true, breaks: true }), {
  ALLOWED_TAGS: ['p','br','strong','em','del','blockquote','ul','ol','li','pre','code','h1','h2','h3','h4','h5','h6','hr','a','table','thead','tbody','tr','th','td'],
  ALLOWED_ATTR: ['href','title','start'],
}).replace(/<a /g, '<a target="_blank" rel="noopener noreferrer" '))
</script>
<style scoped>
.studio-reply { overflow-wrap: anywhere; line-height: 1.85; font-size: 14px; }
.studio-reply :deep(> :first-child) { margin-top: 0; }
.studio-reply :deep(> :last-child) { margin-bottom: 0; }
.studio-reply :deep(p),.studio-reply :deep(ul),.studio-reply :deep(ol),.studio-reply :deep(blockquote),.studio-reply :deep(pre) { margin: .7em 0; }
.studio-reply :deep(ul) { list-style: disc; padding-left: 1.5em; }
.studio-reply :deep(ol) { list-style: decimal; padding-left: 1.5em; }
.studio-reply :deep(h1),.studio-reply :deep(h2),.studio-reply :deep(h3),.studio-reply :deep(h4),.studio-reply :deep(h5),.studio-reply :deep(h6) { margin: 1em 0 .4em; font-size: 1.1em; font-weight: 700; }
.studio-reply :deep(code) { font-size: .88em; padding: .15em .35em; border-radius: 5px; background: rgb(124 144 164 / .12); }
.studio-reply :deep(pre) { max-width: 100%; overflow: auto; padding: 16px; border-radius: 14px; background: rgb(124 144 164 / .1); }
.studio-reply :deep(pre code) { padding: 0; background: none; white-space: pre; }
.studio-reply :deep(a) { color: var(--studio-accent, #219d8b); text-decoration: underline; text-underline-offset: 3px; }
.studio-reply :deep(blockquote) { border-left: 3px solid rgb(67 171 155 / .4); padding-left: 12px; opacity: .8; }
.studio-reply :deep(table) { display: block; max-width: 100%; overflow: auto; border-collapse: collapse; font-size: .9em; }
.studio-reply :deep(th),.studio-reply :deep(td) { border: 1px solid rgb(124 144 164 / .2); padding: 7px 12px; }
</style>
