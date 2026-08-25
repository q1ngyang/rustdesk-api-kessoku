import { defineStore, acceptHMRUpdate } from 'pinia'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import ko from 'element-plus/es/locale/lang/ko'
import ru from 'element-plus/es/locale/lang/ru'
import fr from 'element-plus/es/locale/lang/fr'
import es from 'element-plus/es/locale/lang/es'
import zhTw from 'element-plus/es/locale/lang/zh-tw'
import { admin, app } from '@/api/config'

const langs = {
  'zh-CN': { name: '中文', value: zhCn, sideBarWidth: '210px' },
  'en': { name: 'English', value: en, sideBarWidth: '230px' },
  'fr': { name: 'Français', value: fr, sideBarWidth: '280px' },
  'ko': { name: '한국어', value: ko, sideBarWidth: '230px' },
  'ru': { name: 'Русский', value: ru, sideBarWidth: '250px' },
  'es': { name: 'Español', value: es, sideBarWidth: '280px' },
  'zh-TW': { name: '中文繁体', value: zhTw, sideBarWidth: '210px' },
}
const defaultLang = localStorage.getItem('lang') || navigator.language || 'zh-CN'
export const useAppStore = defineStore({
  id: 'App',
  state: () => ({
    setting: {
      productName: 'RustDesk API Kessoku',
      title: 'RustDesk API Kessoku',
      hello: '',
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
    setLang (lang) {
      this.setting.lang = lang
      this.setting.locale = langs[lang]
      localStorage.setItem('lang', lang)
    },
    changeLang (v) {
      this.setLang(v)
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
      })
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
