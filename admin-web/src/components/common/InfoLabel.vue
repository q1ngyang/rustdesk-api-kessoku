<template>
  <span class="info-label" :class="{ 'info-label--compact': compact }"><span>{{ label }}</span><el-tooltip :content="help" placement="top" :show-after="180"><button type="button" :aria-label="help">?</button></el-tooltip></span>
</template>

<script setup>
defineProps({ label: { type: String, required: true }, help: { type: String, required: true }, compact: Boolean })
</script>

<style scoped>
.info-label { display: inline-flex; width: 100%; min-width: 0; align-items: center; justify-content: space-between; gap: 8px; }
.info-label--compact { width: auto; justify-content: center; gap: 0.45em; }
.info-label button { display: inline-grid; width: 17px; height: 17px; flex: 0 0 auto; padding: 0; place-items: center; border: 1px solid color-mix(in srgb, var(--text-tertiary) 30%, transparent); border-radius: 50%; background: color-mix(in srgb, var(--text-tertiary) 10%, var(--surface-1)); color: var(--text-tertiary); font-size: 10px; font-weight: 800; line-height: 1; cursor: help; }
.info-label button:focus-visible { outline: 2px solid var(--focus-ring); }
</style>
