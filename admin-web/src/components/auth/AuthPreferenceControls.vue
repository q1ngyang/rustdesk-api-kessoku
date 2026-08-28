<template>
  <div class="auth-preferences">
    <el-tooltip :content="isDark ? T('LightMode') : T('DarkMode')" placement="bottom">
      <button class="auth-preferences__button" type="button" :aria-label="isDark ? T('LightMode') : T('DarkMode')" @click="isDark = !isDark">
        <el-icon><Sunny v-if="isDark"/><Moon v-else/></el-icon>
      </button>
    </el-tooltip>
    <el-dropdown trigger="click">
      <button class="auth-preferences__button" type="button" :aria-label="T('ChangeLang')"><el-icon><Guide/></el-icon></button>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item v-for="(language, key) in appStore.setting.langs" :key="key" :class="{ 'is-active-language': key === appStore.setting.lang }" @click="appStore.changeLang(key)">{{ language.name }}</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
  </div>
</template>

<script setup>
import { watch } from 'vue'
import { useDark } from '@vueuse/core'
import { Guide, Moon, Sunny } from '@element-plus/icons-vue'
import { useAppStore } from '@/store/app'
import { T } from '@/utils/i18n'
import { writeSharedPreference } from '@/utils/themeAssets'

const appStore = useAppStore()
const isDark = useDark({ storageKey: 'kessoku-theme' })
watch(isDark, value => writeSharedPreference('kessoku-theme', value ? 'dark' : 'light'))
</script>

<style scoped>
.auth-preferences { position: fixed; top: max(18px, env(safe-area-inset-top)); right: max(18px, env(safe-area-inset-right)); z-index: 8; display: flex; gap: 5px; padding: 4px; border: 1px solid color-mix(in srgb, var(--border-subtle) 78%, transparent); border-radius: 14px; background: color-mix(in srgb, var(--surface-1) 78%, transparent); box-shadow: var(--shadow-sm); backdrop-filter: blur(14px); }
.auth-preferences__button { display: grid; width: 36px; height: 36px; place-items: center; border: 0; border-radius: 10px; background: transparent; color: var(--text-secondary); cursor: pointer; font-size: 16px; }
.auth-preferences__button:hover { background: var(--surface-3); color: var(--primary); }
.auth-preferences__button:focus-visible { outline: 3px solid var(--focus-ring); outline-offset: 1px; }
@media (max-width: 520px) { .auth-preferences { top: max(10px, env(safe-area-inset-top)); right: max(10px, env(safe-area-inset-right)); }.auth-preferences__button { width: 32px; height: 32px; } }
</style>
