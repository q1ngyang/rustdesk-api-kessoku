<template>
  <div class="access-page">
    <div class="page-heading">
      <div><h2>{{ T('AdminAccessScope') }}</h2><p>{{ T('AdminAccessScopeDescription') }}</p></div>
      <el-button @click="router.back()">{{ T('Back') }}</el-button>
    </div>

    <el-card v-loading="loading" shadow="never" class="access-card">
      <div v-if="adminUser.id" class="access-identity">
        <span>{{ (adminUser.username || 'A').slice(0, 1).toUpperCase() }}</span>
        <div><strong>{{ adminUser.username }}</strong><small>{{ adminUser.email || T('EmailNotSet') }}</small></div>
        <el-tag type="warning" effect="light">{{ T('ScopedAdministrator') }}</el-tag>
      </div>

      <el-alert :title="T('ScopeUnionNotice')" :description="T('ScopeUnionDescription')" type="info" :closable="false" show-icon/>

      <el-form label-position="top" class="scope-grid">
        <el-form-item v-for="section in sections" :key="section.key" :label="T(section.label)">
          <el-select
            v-model="scope[section.key]"
            multiple
            filterable
            remote
            reserve-keyword
            collapse-tags
            collapse-tags-tooltip
            :placeholder="T(section.placeholder)"
            :remote-method="query => search(section, query)"
            :loading="section.loading"
            @visible-change="visible => visible && search(section, '')"
          >
            <el-option v-for="item in section.options" :key="optionValue(section, item)" :label="optionLabel(section, item)" :value="optionValue(section, item)"/>
          </el-select>
          <small>{{ T(section.note) }}</small>
        </el-form-item>
      </el-form>

      <div class="access-actions">
        <el-button @click="router.back()">{{ T('Cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ T('SaveAccessScope') }}</el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { detail, options, update } from '@/api/admin_scope'
import { T } from '@/utils/i18n'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const saving = ref(false)
const adminUser = reactive({})
const scope = reactive({ group_ids: [], user_ids: [], collection_ids: [], peer_ids: [] })
const sections = reactive([
  { key: 'group_ids', type: 'group', label: 'ManagedUserGroups', placeholder: 'SearchUserGroups', note: 'ManagedUserGroupsNote', options: [], loading: false, request: 0 },
  { key: 'user_ids', type: 'user', label: 'ManagedUsers', placeholder: 'SearchUsers', note: 'ManagedUsersNote', options: [], loading: false, request: 0 },
  { key: 'collection_ids', type: 'address_book_collection', label: 'ManagedPublicAddressBooks', placeholder: 'SearchPublicAddressBooks', note: 'ManagedPublicAddressBooksNote', options: [], loading: false, request: 0 },
  { key: 'peer_ids', type: 'peer', label: 'ManagedIdDevices', placeholder: 'SearchIdDevices', note: 'ManagedIdDevicesNote', options: [], loading: false, request: 0 },
])

const optionValue = (section, item) => section.type === 'peer' ? item.row_id : item.id
const optionLabel = (section, item) => {
  if (section.type === 'peer') return [item.id, item.hostname].filter(Boolean).join(' · ')
  if (section.type === 'user') return [item.username, item.email].filter(Boolean).join(' · ')
  if (section.type === 'address_book_collection') return `${item.name} · #${item.id}`
  return item.name
}
const mergeOptions = (section, items) => {
  const merged = new Map(section.options.map(item => [optionValue(section, item), item]))
  items.forEach(item => merged.set(optionValue(section, item), item))
  section.options = [...merged.values()]
}
const search = async (section, query) => {
  const requestId = ++section.request
  section.loading = true
  const result = await options({ type: section.type, q: query || '', page: 1, page_size: 50 }).catch(() => false)
  if (requestId !== section.request) return
  section.loading = false
  if (result) mergeOptions(section, result.data.list || [])
}
const load = async () => {
  loading.value = true
  const result = await detail(route.params.id).catch(() => false)
  loading.value = false
  if (!result) return
  Object.assign(adminUser, result.data.admin_user || {})
  Object.assign(scope, result.data.scope || {})
  const selected = {
    group_ids: result.data.groups || [], user_ids: result.data.users || [],
    collection_ids: result.data.collections || [], peer_ids: result.data.peers || [],
  }
  sections.forEach(section => mergeOptions(section, selected[section.key]))
}
const save = async () => {
  saving.value = true
  const result = await update({ user_id: Number(route.params.id), ...scope }).catch(() => false)
  saving.value = false
  if (!result) return
  ElMessage.success(T('OperationSuccess'))
  router.back()
}
onMounted(load)
</script>

<style scoped lang="scss">
.access-card { border-radius: 18px; }.access-identity { display: flex; align-items: center; gap: 12px; margin-bottom: 18px; }.access-identity > span { display: grid; width: 46px; height: 46px; place-items: center; border-radius: 14px; background: linear-gradient(145deg, var(--primary), #8467ef); color: #fff; font-weight: 800; }.access-identity > div { display: flex; min-width: 0; flex: 1; flex-direction: column; }.access-identity strong { color: var(--text-primary); }.access-identity small { margin-top: 3px; color: var(--text-tertiary); }.scope-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 4px 18px; margin-top: 22px; }.scope-grid .el-select { width: 100%; }.scope-grid small { display: block; margin-top: 7px; color: var(--text-tertiary); line-height: 1.55; }.access-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 8px; padding-top: 18px; border-top: 1px solid var(--border-subtle); }
@media (max-width: 720px) { .scope-grid { grid-template-columns: 1fr; }.access-identity { align-items: flex-start; flex-wrap: wrap; }.access-identity > div { min-width: calc(100% - 60px); }.access-actions .el-button { flex: 1; } }
</style>
