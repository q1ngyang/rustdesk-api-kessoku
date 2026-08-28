<template>
  <footer class="brand-footer" v-html="safeHTML"></footer>
</template>

<script setup>
import { computed } from 'vue'
import DOMPurify from 'dompurify'

const props = defineProps({ html: { type: String, default: '' } })
const defaultFooter = '<a href="https://github.com/q1ngyang/rustdesk-api-kessoku" target="_blank" rel="noopener noreferrer"><span>RustDesk API Kessoku</span><span>Github</span></a>'

const safeHTML = computed(() => {
  const sanitized = DOMPurify.sanitize(props.html || defaultFooter, {
    ALLOWED_TAGS: ['a', 'span', 'strong', 'em'],
    ALLOWED_ATTR: ['href', 'target', 'rel', 'aria-label'],
  })
  const template = document.createElement('template')
  template.innerHTML = sanitized
  template.content.querySelectorAll('a').forEach(link => {
    try {
      const target = new URL(link.getAttribute('href') || '', window.location.href)
      if (target.protocol !== 'https:') throw new Error('unsupported protocol')
      link.href = target.href
      link.target = '_blank'
      link.rel = 'noopener noreferrer'
    } catch { link.removeAttribute('href') }
  })
  return template.innerHTML
})
</script>

<style scoped>
.brand-footer { display: flex; min-height: 24px; align-items: center; justify-content: center; color: var(--text-tertiary); font-size: 10px; font-weight: 620; line-height: 1.5; text-align: center; }
.brand-footer :deep(a) { display: inline-flex; align-items: center; gap: 10px; color: inherit; text-decoration: none; transition: color var(--motion-fast); }
.brand-footer :deep(a:hover) { color: var(--text-secondary); }
.brand-footer :deep(a span + span) { padding-left: 10px; border-left: 1px solid var(--border-subtle); }
</style>
