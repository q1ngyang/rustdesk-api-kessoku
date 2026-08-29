<template>
  <span class="date-time-cell"><span>{{ parts.date }}</span><small v-if="parts.time">{{ parts.time }}</small></span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({ value: { type: [String, Number, Date], default: '' }, unix: Boolean })
const parts = computed(() => {
  if (props.value === '' || props.value === null || props.value === undefined) return { date: '-', time: '' }
  let raw = props.value
  if (props.unix) {
    const seconds = Number(props.value)
    if (!Number.isFinite(seconds) || seconds <= 0) return { date: '-', time: '' }
    raw = seconds * 1000
  }
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return { date: '-', time: '' }
  return {
    date: date.toLocaleDateString(undefined, { year: 'numeric', month: '2-digit', day: '2-digit' }),
    time: date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }),
  }
})
</script>

<style scoped>
.date-time-cell { display: inline-flex; min-width: 112px; align-items: center; flex-direction: column; color: var(--text-secondary); font-size: 11px; line-height: 1.35; white-space: nowrap; }
.date-time-cell small { margin-top: 2px; color: var(--text-tertiary); font-size: 10px; }
</style>
