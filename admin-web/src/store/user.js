import { defineStore, acceptHMRUpdate } from 'pinia'
import { current, login } from '@/api/user'
import { setToken, removeToken, setCode, removeCode, getOidcClientBinding } from '@/utils/auth'
import { useRouteStore } from '@/store/router'
import { useAppStore } from '@/store/app'
import { oidcAuth, oidcQuery } from '@/api/login'
import { browserDeviceIdentity } from '@/utils/device'

export const useUserStore = defineStore({
  id: 'user',
  state: () => ({
    nickname: '',
    username: '',
    email: '',
    token: '',
    role: '',
    avatar: '',
    preference_language: '',
    preference_theme: '',
    route_names: [],
  }),

  actions: {
    logout () {
      removeToken()
      removeCode()
      useRouteStore().resetRoutes()
      this.$patch({
        nickname: '',
        username: '',
        email: '',
        token: '',
        role: '',
        avatar: '',
        preference_language: '',
        preference_theme: '',
        route_names: [],
      })
    },

    saveUserData (userData) {
      // useAppStore().getAppConfig()
      setToken(userData.token)
      //
      localStorage.setItem('user_info', JSON.stringify({ name: userData.username }))
      this.$patch({
        ...userData,
      })
      useAppStore().applyUserPreferences(userData)
      if (userData.route_names && userData.route_names.length) {
        useRouteStore().addRoutes(userData.route_names)
      }
    },

    async login (form) {
      const res = await login(form).catch(e => e)
      if (!res.code) {
		if (res.data?.requires_two_factor) return res.data
        useAppStore().loadConfig()
        const userData = res.data
        this.saveUserData(userData)
        return userData
      } else {
        return Promise.reject(res)
      }
    },
    async info () {
      const res = await current().catch(_ => false)
      if (res) {
        useAppStore().loadConfig()
        const userData = res.data
        this.saveUserData(userData)
        return userData
      }
      return false
    },
    async oidc (provider, platform, browser) {
      const identity = browserDeviceIdentity()
      const data = {
        deviceInfo: {
          name: identity.device_id,
          os: platform,
          type: 'webadmin',
        },
        id: identity.device_id || `${platform}-${browser}`,
        op: provider,
        uuid: identity.uuid,
      }
      const res = await oidcAuth(data).catch(_ => false)
      if (res) {
        const { code, url } = res.data
		setCode(code, data.id, data.uuid)
        if (provider == 'webauth') {
          const popup = window.open(url, '_blank', 'noopener,noreferrer')
          if (popup) popup.opener = null
        } else {
          window.location.href = url
        }
      }
    },
    async query (code) {
		const binding = getOidcClientBinding()
		const params = { code, id: binding.id, uuid: binding.uuid }
      const res = await oidcQuery(params).catch(_ => false)
      if (res) {
        removeCode()
        useAppStore().loadConfig()
        const userData = res.data
        this.saveUserData(userData)
        return userData
      }
      return false
    },
  },
})

if (import.meta.hot) {
  import.meta.hot.accept(acceptHMRUpdate(useUserStore, import.meta.hot))
}
