<template>
  <span class="username-cell">{{ value || '-' }}</span>
</template>

<script setup>
defineProps({ value: { type: String, default: '' } })
</script>

<style scoped>
.username-cell {
  display: inline-block;
  max-width: 10em;
  color: var(--text-primary);
  font-size: 12px;
  font-weight: 650;
  line-height: 1.4;
  overflow-wrap: anywhere;
  text-align: center;
  white-space: normal;
  word-break: break-word;
}
</style>
