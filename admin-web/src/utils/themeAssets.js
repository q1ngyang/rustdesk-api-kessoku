import iconDark from '@/assets/brand/starrydesk-icon-dark.svg'
import iconLight from '@/assets/brand/starrydesk-icon-light.svg'

const THEME_KEY = 'kessoku-theme'

export function readSharedPreference (name) {
  try {
    const prefix = `${encodeURIComponent(name)}=`
    const item = document.cookie.split(';').map(value => value.trim()).find(value => value.startsWith(prefix))
    return item ? decodeURIComponent(item.slice(prefix.length)) : ''
  } catch {
    return ''
  }
}

export function writeSharedPreference (name, value) {
  try {
    const secure = window.location.protocol === 'https:' ? '; Secure' : ''
    document.cookie = `${encodeURIComponent(name)}=${encodeURIComponent(value)}; Path=/; Max-Age=31536000; SameSite=Lax${secure}`
  } catch { /* Browser privacy settings may disable non-essential cookies. */ }
}

function storedTheme () {
  try {
    const value = readSharedPreference(THEME_KEY) || localStorage.getItem(THEME_KEY)
    return value === 'dark' || value === 'light' ? value : ''
  } catch {
    return ''
  }
}

export function syncThemeAssets () {
  const dark = document.documentElement.classList.contains('dark')
  const favicon = document.querySelector('#kessoku-favicon')
  const themeColor = document.querySelector('meta[name="theme-color"]')
  if (favicon) favicon.setAttribute('href', (dark ? favicon.dataset.deploymentIconDark : favicon.dataset.deploymentIconLight) || (dark ? iconDark : iconLight))
  if (themeColor) themeColor.setAttribute('content', dark ? '#10131a' : '#f5f7fb')
}

export function setDeploymentFavicon (lightUrl = '', darkUrl = '') {
  const favicon = document.querySelector('#kessoku-favicon')
  if (!favicon) return
  if (lightUrl) favicon.dataset.deploymentIconLight = lightUrl
  else delete favicon.dataset.deploymentIconLight
  if (darkUrl) favicon.dataset.deploymentIconDark = darkUrl
  else delete favicon.dataset.deploymentIconDark
  syncThemeAssets()
}

export function initializeThemeAssets () {
  const saved = storedTheme()
  if (saved) {
    try { localStorage.setItem(THEME_KEY, saved) } catch { /* Keep the cookie-backed value when storage is unavailable. */ }
  }
  const prefersDark = window.matchMedia?.('(prefers-color-scheme: dark)').matches === true
  document.documentElement.classList.toggle('dark', saved ? saved === 'dark' : prefersDark)
  syncThemeAssets()

  const observer = new MutationObserver(syncThemeAssets)
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
}
