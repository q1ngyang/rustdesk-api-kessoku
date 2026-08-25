<template>
  <div class="kessoku-brand" :class="{ 'kessoku-brand--compact': compact }">
    <StarryIcon :size="compact ? 34 : 38" alt=""/>
    <div v-if="!compact" class="kessoku-brand__copy">
      <strong>RustDesk API</strong>
      <span>KESSOKU</span>
    </div>
  </div>
</template>

<script setup>
import StarryIcon from '@/components/brand/StarryIcon.vue'
defineProps({ compact: Boolean })
</script>

<style scoped lang="scss">
.kessoku-brand { display: flex; min-width: 0; align-items: center; gap: 12px; }
.kessoku-brand__copy { display: flex; min-width: 0; flex-direction: column; line-height: 1.15; }
.kessoku-brand__copy strong { overflow: hidden; color: var(--text-primary); font-size: 14px; font-weight: 760; letter-spacing: -.02em; text-overflow: ellipsis; white-space: nowrap; }
.kessoku-brand__copy span { margin-top: 4px; color: var(--text-tertiary); font-size: 11px; font-weight: 650; letter-spacing: .12em; text-transform: uppercase; }
</style>
