<template>
  <div class="tags-scroll">
    <button v-for="tag in tags" :key="tag.name" class="route-tag" :class="{ 'is-active': tag.active }" type="button" @click="toTag(tag)">
      <span>{{ T(tag.title) }}</span><el-icon v-if="tag.closeable" class="route-tag__close" @click.stop="close(tag)"><Close/></el-icon>
    </button>
  </div>
</template>

<script setup>
import { onMounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import { Close } from '@element-plus/icons-vue'
import { useTagsStore } from '@/store/tags'
import { T } from '@/utils/i18n'

const tagsStore = useTagsStore()
const { tags } = storeToRefs(tagsStore)
const route = useRoute()
const router = useRouter()
const addTag = value => { if (!value.meta?.hide && value.name) tagsStore.addTag(value) }
const toLastTag = () => { if (tags.value.length) router.push({ name: tags.value[tags.value.length - 1].name }) }
const close = tag => { tagsStore.removeTag(tag); if (tag.active) toLastTag() }
const toTag = tag => { if (tag.name !== route.name) router.push({ name: tag.name }) }
onMounted(() => { if (!tags.value.length) tagsStore.initTags(); addTag(route) })
watch(route, addTag)
</script>

<style scoped lang="scss">
.tags-scroll { display: flex; min-width: 0; gap: 6px; overflow-x: auto; padding: 6px clamp(18px, 2vw, 28px); scrollbar-width: none; }.tags-scroll::-webkit-scrollbar { display: none; }
.route-tag { display: inline-flex; height: 26px; flex: 0 0 auto; align-items: center; gap: 5px; padding: 0 9px; border: 1px solid transparent; border-radius: 8px; background: transparent; color: var(--text-tertiary); font-size: 11px; font-weight: 650; cursor: pointer; }
.route-tag:hover { background: var(--surface-3); color: var(--text-primary); }.route-tag.is-active { border-color: var(--primary-border); background: var(--primary-soft); color: var(--primary); }.route-tag__close { border-radius: 50%; font-size: 11px; }.route-tag__close:hover { background: color-mix(in srgb, var(--primary) 14%, transparent); }
</style>
