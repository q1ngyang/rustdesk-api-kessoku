<template><span class="status-value" :class="`status-value--${tone}`"><i aria-hidden="true"></i><span>{{ display }}</span></span></template>

<script setup>
import { computed } from 'vue'
const props = defineProps({ value: { type: [String, Boolean, Number], default: '-' }, text: { type: String, default: '' } })
const normalized = computed(() => String(props.value ?? '-').trim().toLowerCase())
const display = computed(() => props.text || (props.value === true ? 'true' : props.value === false ? 'false' : String(props.value ?? '-')))
const tone = computed(() => {
  if (['active', 'true', 'online', 'healthy', 'available', 'ready', 'succeeded', 'enabled', 'valid'].includes(normalized.value)) return 'success'
  if (['pending', 'running', 'degraded', 'warning', 'read-only', 'readonly'].includes(normalized.value)) return 'warning'
  if (['false', 'offline', 'unhealthy', 'failed', 'error', 'disabled', 'unavailable', 'invalid'].includes(normalized.value)) return 'danger'
  return 'neutral'
})
</script>

<style scoped>
.status-value { display: inline-flex; align-items: center; gap: 7px; color: var(--text-secondary); font-weight: 650; }
.status-value i { width: 8px; height: 8px; flex: 0 0 auto; border-radius: 50%; background: var(--text-tertiary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--text-tertiary) 13%, transparent); }
.status-value--success i { background: var(--success); box-shadow: 0 0 0 3px var(--success-soft); }
.status-value--warning i { background: var(--warning); box-shadow: 0 0 0 3px color-mix(in srgb, var(--warning) 14%, transparent); }
.status-value--danger i { background: var(--danger); box-shadow: 0 0 0 3px var(--danger-soft); }
</style>
