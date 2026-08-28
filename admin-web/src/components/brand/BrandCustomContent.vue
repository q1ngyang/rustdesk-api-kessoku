<template><div ref="host" class="brand-custom-host" :hidden="!html"></div></template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import DOMPurify from 'dompurify'

const props = defineProps({ html: { type: String, default: '' }, css: { type: String, default: '' } })
const host = ref(null)
let shadow
const render = () => {
  if (!host.value) return
  shadow ||= host.value.attachShadow({ mode: 'closed' })
  const safe = DOMPurify.sanitize(props.html, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'link', 'meta', 'form', 'input', 'button', 'textarea', 'select', 'img', 'video', 'audio'],
    FORBID_ATTR: ['style', 'src', 'srcset', 'href', 'action', 'formaction'],
  })
  shadow.replaceChildren()
  const style = document.createElement('style')
  style.textContent = `:host{display:block}.brand-custom{color:var(--text-secondary);font:inherit;line-height:1.65}${props.css}`
  const content = document.createElement('div')
  content.className = 'brand-custom'
  content.innerHTML = safe
  shadow.append(style, content)
}
onMounted(render)
watch(() => [props.html, props.css], render)
</script>

<style scoped>.brand-custom-host[hidden] { display: none; }</style>
