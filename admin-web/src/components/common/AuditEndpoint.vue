<template>
  <div class="audit-endpoint">
    <strong>{{ id || '-' }}</strong>
    <small v-if="username" class="audit-endpoint__username"><span aria-hidden="true">@</span><span>{{ username }}</span></small>
    <IpAddress v-if="ip" :value="ip"/>
    <small v-else class="audit-endpoint__empty">{{ T('IPUnavailable') }}</small>
  </div>
</template>

<script setup>
import IpAddress from './IpAddress.vue'
import { T } from '@/utils/i18n'
defineProps({ id: { type: String, default: '' }, ip: { type: String, default: '' }, username: { type: String, default: '' } })
</script>

<style scoped>
.audit-endpoint { display: flex; min-width: 150px; align-items: flex-start; flex-direction: column; gap: 3px; text-align: left; }.audit-endpoint strong { max-width: 100%; overflow: hidden; color: var(--text-primary); font-family: var(--font-mono, ui-monospace, monospace); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.audit-endpoint small { color: var(--primary); font-size: 10px; line-height: 1.4; }.audit-endpoint__username { display: inline-flex; max-width: calc(10em + 1ch); align-items: flex-start; }.audit-endpoint__username > span:first-child { flex: 0 0 auto; }.audit-endpoint__username > span:last-child { max-width: 10em; overflow-wrap: anywhere; white-space: normal; word-break: break-word; }.audit-endpoint .audit-endpoint__empty { color: var(--text-tertiary); }
</style>
