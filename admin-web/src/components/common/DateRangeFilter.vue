<template>
  <el-date-picker
    :model-value="modelValue"
    type="daterange"
    value-format="YYYY-MM-DD"
    unlink-panels
    clearable
    :range-separator="T('To')"
    :start-placeholder="T('StartDate')"
    :end-placeholder="T('EndDate')"
    @update:model-value="$emit('update:modelValue', $event || [])"
  />
</template>

<script setup>
import { T } from '@/utils/i18n'
defineProps({ modelValue: { type: Array, default: () => [] } })
defineEmits(['update:modelValue'])
</script>

<style scoped>
:deep(.el-date-editor) { width: 270px; max-width: 100%; }
</style>
