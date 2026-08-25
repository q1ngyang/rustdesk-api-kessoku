import iconDark from '@/assets/brand/starrylinks-icon-dark.svg'
import iconLight from '@/assets/brand/starrylinks-icon-light.svg'

const THEME_KEY = 'kessoku-theme'

function storedTheme () {
  try {
    const value = localStorage.getItem(THEME_KEY)
    return value === 'dark' || value === 'light' ? value : ''
  } catch {
    return ''
  }
}

export function syncThemeAssets () {
  const dark = document.documentElement.classList.contains('dark')
  const favicon = document.querySelector('#kessoku-favicon')
  const themeColor = document.querySelector('meta[name="theme-color"]')
  if (favicon) favicon.setAttribute('href', dark ? iconDark : iconLight)
  if (themeColor) themeColor.setAttribute('content', dark ? '#10131a' : '#f5f7fb')
}

export function initializeThemeAssets () {
  const saved = storedTheme()
  const prefersDark = window.matchMedia?.('(prefers-color-scheme: dark)').matches === true
  document.documentElement.classList.toggle('dark', saved ? saved === 'dark' : prefersDark)
  syncThemeAssets()

  const observer = new MutationObserver(syncThemeAssets)
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
}
