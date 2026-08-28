import { defineStore, acceptHMRUpdate } from 'pinia'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import ko from 'element-plus/es/locale/lang/ko'
import ru from 'element-plus/es/locale/lang/ru'
import fr from 'element-plus/es/locale/lang/fr'
import es from 'element-plus/es/locale/lang/es'
import zhTw from 'element-plus/es/locale/lang/zh-tw'
import ja from 'element-plus/es/locale/lang/ja'
import { admin, app } from '@/api/config'
import { updatePreferences } from '@/api/user'
import { getToken } from '@/utils/auth'
import { readSharedPreference, setDeploymentFavicon, syncThemeAssets, writeSharedPreference } from '@/utils/themeAssets'

const langs = {
  'zh-CN': { name: '简体中文', value: zhCn, sideBarWidth: '210px' },
  'zh-TW': { name: '繁体中文', value: zhTw, sideBarWidth: '210px' },
  'en': { name: 'English', value: en, sideBarWidth: '230px' },
  'ja': { name: '日本語', value: ja, sideBarWidth: '240px' },
  'ko': { name: '한국어', value: ko, sideBarWidth: '230px' },
  'fr': { name: 'Français', value: fr, sideBarWidth: '280px' },
  'es': { name: 'Español', value: es, sideBarWidth: '280px' },
  'ru': { name: 'Русский', value: ru, sideBarWidth: '250px' },
}
const browserLanguage = (navigator.language || '').toLowerCase()
const detectedLanguage = browserLanguage.startsWith('zh-tw') || browserLanguage.startsWith('zh-hk') ? 'zh-TW'
  : browserLanguage.startsWith('zh') ? 'zh-CN'
    : ['ja', 'ko', 'ru', 'fr', 'es'].find(lang => browserLanguage.startsWith(lang)) || 'en'
const storedLanguage = readSharedPreference('kessoku-language') || localStorage.getItem('lang')
const defaultLang = storedLanguage && langs[storedLanguage] ? storedLanguage : detectedLanguage
export const useAppStore = defineStore({
  id: 'App',
  state: () => ({
    setting: {
      productName: 'RustDesk API Kessoku',
      title: 'RustDesk API Kessoku',
      hello: '',
	  branding: {
		admin_title: '', admin_subtitle: '', brand_logo_light_url: '', brand_logo_dark_url: '', brand_icon_light_url: '', brand_icon_dark_url: '',
		login_background_light_url: '', login_background_dark_url: '', web_client_background_light_url: '', web_client_background_dark_url: '',
		login_kicker: '', login_heading: '', login_copy: '', footer_html: '', login_custom_html: '', login_custom_css: '', web_client_title: '',
	  },
      sideIsCollapse: false,
      mobileSidebarOpen: false,
      viewportWidth: window.innerWidth,
      langs: langs,
      lang: defaultLang,
      locale: langs[defaultLang] ? langs[defaultLang] : langs['en'],
      appConfig: {
        web_client_mode: 'disabled',
        web_client_public_origin: '',
      },
    },
  }),

  actions: {
    sideCollapse () {
      this.setting.sideIsCollapse = !this.setting.sideIsCollapse
    },
    openMobileSidebar () {
      this.setting.mobileSidebarOpen = true
    },
    closeMobileSidebar () {
      this.setting.mobileSidebarOpen = false
    },
    setViewportWidth (width) {
      this.setting.viewportWidth = width
    },
    setLang (lang, syncAccount = true) {
      const normalized = langs[lang] ? lang : 'en'
      this.setting.lang = normalized
      this.setting.locale = langs[normalized]
      localStorage.setItem('lang', normalized)
      writeSharedPreference('kessoku-language', normalized)
      if (syncAccount && getToken()) void updatePreferences({ language: normalized }).catch(() => undefined)
    },
    changeLang (v) {
      this.setLang(v)
    },
    applyUserPreferences (userData = {}) {
      const language = userData.preference_language
      if (language && langs[language]) this.setLang(language, false)
      const theme = userData.preference_theme
      if (theme === 'light' || theme === 'dark') {
        localStorage.setItem('kessoku-theme', theme)
        writeSharedPreference('kessoku-theme', theme)
        document.documentElement.classList.toggle('dark', theme === 'dark')
        syncThemeAssets()
        window.dispatchEvent(new CustomEvent('kessoku:account-theme', { detail: { theme } }))
      }
    },
    loadConfig () {
      this.getAppConfig()
      this.getAdminConfig()
    },
    getAppConfig () {
      return app().then(res => {
        this.setting.appConfig = res.data
      })
    },
    getAdminConfig () {
      return admin().then(res => {
        this.replaceAdminTitle(res.data.title)
        this.setting.hello = res.data.hello
		this.setting.branding = { ...this.setting.branding, ...(res.data.branding || {}) }
		this.applyBrandingDocument()
      })
    },
	applyBrandingDocument () {
	  const branding = this.setting.branding
	  setDeploymentFavicon(branding.brand_icon_light_url, branding.brand_icon_dark_url)
	},
    replaceAdminTitle (newTitle) {
      if (!newTitle) return
      document.title = document.title.replace(`- ${this.setting.title}`, `- ${newTitle}`)
      this.setting.title = newTitle
    },
  },
})

if (import.meta.hot) {
  import.meta.hot.accept(acceptHMRUpdate(useAppStore, import.meta.hot))
}
