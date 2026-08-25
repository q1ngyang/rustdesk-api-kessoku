<template>
  <div class="setting">
    <el-tooltip :content="isDark ? T('LightMode') : T('DarkMode')" placement="bottom">
      <button class="setting__icon" type="button" :aria-label="isDark ? T('LightMode') : T('DarkMode')" @click="isDark = !isDark"><el-icon><Sunny v-if="isDark"/><Moon v-else/></el-icon></button>
    </el-tooltip>
    <el-dropdown trigger="click">
      <button class="setting__icon" type="button" :aria-label="T('ChangeLang')"><el-icon><Guide/></el-icon></button>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item v-for="(language, key) in appStore.setting.langs" :key="key" :class="{ 'is-active-language': key === appStore.setting.lang }" @click="changeLang(key)">{{ language.name }}</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
    <el-dropdown trigger="click">
      <button class="setting__user" type="button">
        <span class="setting__avatar">{{ initial }}</span><span class="setting__name">{{ user.username }}</span><el-icon><ArrowDown/></el-icon>
      </button>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item @click="toProfile"><el-icon><User/></el-icon>{{ T('Userinfo') }}</el-dropdown-item>
          <el-dropdown-item @click="showChangePwd"><el-icon><Lock/></el-icon>{{ T('ChangePassword') }}</el-dropdown-item>
          <el-dropdown-item @click="toAbout"><el-icon><InfoFilled/></el-icon>{{ T('About') }}</el-dropdown-item>
          <el-dropdown-item divided @click="logout"><el-icon><SwitchButton/></el-icon>{{ T('Logout') }}</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
    <changePwdDialog v-model:visible="changePwdVisible"/>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useDark } from '@vueuse/core'
import { ArrowDown, Guide, InfoFilled, Lock, Moon, Sunny, SwitchButton, User } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import { useAppStore } from '@/store/app'
import changePwdDialog from '@/components/changePwdDialog.vue'
import { T } from '@/utils/i18n'

const user = useUserStore()
const appStore = useAppStore()
const router = useRouter()
const changePwdVisible = ref(false)
const isDark = useDark({ storageKey: 'kessoku-theme' })
const initial = computed(() => (user.username || 'K').slice(0, 1).toUpperCase())
const logout = () => { user.logout(); window.location.reload() }
const showChangePwd = () => { changePwdVisible.value = true }
const changeLang = value => appStore.changeLang(value)
const toProfile = () => router.push({ name: 'MyInfo' })
const toAbout = () => router.push({ name: 'About' })
</script>

<style scoped lang="scss">
.setting { display: flex; min-width: 0; align-items: center; gap: 6px; margin-left: auto; }
.setting__icon,.setting__user { border: 0; background: transparent; color: var(--text-secondary); cursor: pointer; }
.setting__icon { display: grid; width: 38px; height: 38px; place-items: center; border-radius: 12px; font-size: 17px; }
.setting__icon:hover,.setting__user:hover { background: var(--surface-3); color: var(--primary); }.setting__icon:focus-visible,.setting__user:focus-visible { outline: 3px solid var(--focus-ring); outline-offset: 1px; }
.setting__user { display: flex; min-width: 0; height: 42px; align-items: center; gap: 8px; padding: 4px 8px 4px 5px; border-radius: 14px; }
.setting__avatar { display: grid; width: 32px; height: 32px; flex: 0 0 auto; place-items: center; border-radius: 10px; background: linear-gradient(145deg, var(--primary), #8467ef); color: white; font-size: 12px; font-weight: 800; }
.setting__name { max-width: 120px; overflow: hidden; color: var(--text-primary); font-size: 12px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 600px) { .setting { gap: 1px; }.setting__name,.setting__user > .el-icon { display: none; }.setting__user { width: 38px; padding: 3px; }.setting__icon { width: 34px; } }
</style>
