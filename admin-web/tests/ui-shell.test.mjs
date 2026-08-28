import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const source = path => readFile(new URL(`../${path}`, import.meta.url), 'utf8')

test('v3 shell keeps every permission-filtered route reachable on small screens', async () => {
  const [layout, bottomNav, menu, menuItem] = await Promise.all([
    source('src/layout/index.vue'),
    source('src/layout/components/MobileBottomNav.vue'),
    source('src/layout/components/menu/index.vue'),
    source('src/layout/components/menu/item.vue'),
  ])
  assert.match(layout, /mobileSidebarOpen/)
  assert.match(layout, /<g-aside/)
  assert.match(layout, /<mobile-bottom-nav/)
  assert.match(bottomNav, /routeStore\.routes/)
  assert.match(bottomNav, /openMobileSidebar/)
  assert.match(menu, /routeStore\.routes/)
  assert.match(menuItem, /<Item v-for="child in visibleChildren"/)
  assert.doesNotMatch(menuItem, /<menu-item v-for="child in visibleChildren"/)
})

test('responsive styles preserve tablet expansion and mobile table actions', async () => {
  const [layout, styles] = await Promise.all([
    source('src/layout/index.vue'),
    source('src/styles/style.scss'),
  ])
  assert.match(layout, /enteredTablet/)
  assert.doesNotMatch(layout, /@media \(max-width: 1100px\)[\s\S]*grid-template-columns: var\(--sidebar-collapsed-width\)/)
  assert.match(styles, /\.el-table-fixed-column--right \{ position: static !important;/)
  assert.match(styles, /env\(safe-area-inset-bottom\)/)
})

test('Kessoku visual accents and supplied brand assets stay scoped and theme aware', async () => {
  const [tokens, brand, deploymentAsset, themedAsset, themeAssets, customBrand, auth, authAction, header, overview, login, serverControl, about] = await Promise.all([
    source('src/styles/style.scss'),
    source('src/components/brand/KessokuBrand.vue'),
    source('src/components/brand/DeploymentAsset.vue'),
    source('src/components/brand/ThemeBrandAsset.vue'),
    source('src/utils/themeAssets.js'),
    source('src/components/brand/BrandCustomContent.vue'),
    source('src/components/auth/AuthLayout.vue'),
    source('src/components/auth/AuthActionLayout.vue'),
    source('src/layout/components/header.vue'),
    source('src/views/my/info.vue'),
    source('src/views/login/login.vue'),
    source('src/views/server_control/index.vue'),
    source('src/views/about/index.vue'),
  ])
  for (const color of ['blue', 'yellow', 'red', 'pink']) assert.match(tokens, new RegExp(`--kessoku-${color}:`))
  assert.match(brand, /<DeploymentAsset/)
  assert.match(brand, /RustDesk API/)
  assert.match(brand, /KESSOKU/)
  assert.match(deploymentAsset, /starrydesk-logo-light\.svg/)
  assert.match(deploymentAsset, /starrydesk-logo-dark\.svg/)
  assert.match(deploymentAsset, /starrydesk-icon-light\.svg/)
  assert.match(deploymentAsset, /starrydesk-icon-dark\.svg/)
  assert.match(deploymentAsset, /useDark/)
  assert.match(deploymentAsset, /activeSource/)
  assert.match(themedAsset, /starrylinks-logo-light\.svg/)
  assert.match(themedAsset, /starrylinks-logo-dark\.svg/)
  assert.match(themedAsset, /useDark/)
  assert.match(themedAsset, /activeSource/)
  assert.match(themeAssets, /dark \? iconDark : iconLight/)
  assert.match(themeAssets, /dataset\.deploymentIconLight/)
  assert.match(themeAssets, /dataset\.deploymentIconDark/)
  assert.match(themeAssets, /setDeploymentFavicon/)
  assert.match(customBrand, /attachShadow\(\{ mode: 'closed' \}\)/)
  assert.match(customBrand, /:hidden="!html"/)
  assert.doesNotMatch(customBrand, /:empty/)
  assert.doesNotMatch(auth, /auth-story__palette/)
  assert.doesNotMatch(authAction, /action-panel__palette/)
  assert.doesNotMatch(header, /topbar__accent/)
  assert.match(overview, /quick-card__icon--pink/)
  assert.match(overview, /quick-card__icon--yellow/)
  assert.match(login, /show-starry-logo/)
  assert.match(serverControl, /<StarryLogo/)
  assert.match(about, /<DeploymentAsset/)
  assert.doesNotMatch(authAction, /StarryLogo/)
})

test('about remains an authenticated front-end page without widening backend route permissions', async () => {
  const [routes, permission, settings] = await Promise.all([
    source('src/router/index.js'),
    source('src/permission.js'),
    source('src/layout/components/setting/index.vue'),
  ])
  assert.match(routes, /path: '\/about'/)
  assert.match(routes, /name: 'About'/)
  assert.doesNotMatch(permission, /whiteList = \[[^\]]*about/)
  assert.match(settings, /toAbout/)
})

test('enterprise role UI separates scoped administration from super administration', async () => {
  const [routes, routeStore, users, access, mobile, addressBook, tags, peers] = await Promise.all([
    source('src/router/index.js'),
    source('src/store/router.js'),
    source('src/views/user/index.vue'),
    source('src/views/user/access.vue'),
    source('src/layout/components/MobileBottomNav.vue'),
    source('src/views/address_book/index.vue'),
    source('src/views/tag/index.vue'),
    source('src/views/peer/index.vue'),
  ])
  assert.match(routes, /name: 'AdminAccess'/)
  assert.match(routeStore, /resetRoutes \(\)/)
  assert.match(routeStore, /router\.removeRoute/)
  assert.match(users, /row\.role === 'admin'/)
  assert.match(users, /isSuperAdmin && row\.role/)
  for (const key of ['group_ids', 'user_ids', 'collection_ids', 'peer_ids']) assert.match(access, new RegExp(key))
  assert.match(mobile, /\['admin', 'super_admin'\]\.includes\(userStore\.role\)/)
  assert.match(addressBook, /v-if="isSuperAdmin" :value="0"/)
  assert.match(tags, /v-if="isSuperAdmin" :value="0"/)
  assert.match(peers, /v-if="isSuperAdmin" :label="T\('Owner'\)"/)
})

test('super-administrator settings use a dedicated bottom navigation group', async () => {
  const [routes, announcement, geoip, branding, login, webClient] = await Promise.all([
    source('src/router/index.js'),
    source('src/views/settings/announcement.vue'),
    source('src/views/settings/geoip.vue'),
    source('src/views/branding/index.vue'),
    source('src/components/auth/AuthLayout.vue'),
    readFile(new URL('../../web-client/src/main.ts', import.meta.url), 'utf8'),
  ])
  assert.match(routes, /name: 'SystemManagement'/)
  assert.match(routes, /name: 'AnnouncementSettings'/)
  assert.match(routes, /name: 'GeoIPSettings'/)
  assert.match(routes, /meta: \{ title: 'ConnectionManagement'/)
  assert.match(routes, /path: '\/branding', redirect: '\/system\/branding'/)
  assert.match(announcement, /saveSystemSettingForm/)
  assert.match(geoip, /geoip_country_url/)
  assert.match(geoip, /monitorUpdate\(true\)/)
  assert.match(geoip, /GeoIPUpdateSucceeded/)
  assert.match(geoip, /GeoIPUpdateFailed/)
  assert.match(branding, /FooterHTML/)
  assert.match(login, /<BrandFooter/)
  assert.match(login, /login_background_dark_url/)
  assert.match(branding, /brand_logo_light_url/)
  assert.match(branding, /web_client_background_dark_url/)
  assert.match(webClient, /panelBrandLogo/)
  assert.match(webClient, /event\.ctrlKey/)
})

test('overview, server tabs and WebClient controls preserve the reviewed compact layout', async () => {
  const [overview, serverControl, webClient, webStyles] = await Promise.all([
    source('src/views/my/info.vue'),
    source('src/views/server_control/index.vue'),
    readFile(new URL('../../web-client/src/main.ts', import.meta.url), 'utf8'),
    readFile(new URL('../../web-client/src/styles.css', import.meta.url), 'utf8'),
  ])
  assert.match(overview, /@container \(max-width: 400px\)/)
  assert.match(overview, /\.signed-in-card \{[^}]*justify-self: end;[^}]*margin-top: 0;/)
  assert.match(serverControl, /\.el-tabs__nav-scroll\) \{ padding-left: 12px; \}/)
  assert.match(webClient, /remotePassword\.wrap\.hidden = active/)
  assert.match(webClient, /registeredRemoteHostname/)
  assert.match(webStyles, /\.panel-brand-logo \{[^}]*width: 169px;[^}]*margin-right: -8px;/)
})

test('language and display preferences synchronize through the signed-in account', async () => {
  const [appStore, userStore, settings, api, webClient, webAuth] = await Promise.all([
    source('src/store/app.js'),
    source('src/store/user.js'),
    source('src/layout/components/setting/index.vue'),
    source('src/api/user.js'),
    readFile(new URL('../../web-client/src/main.ts', import.meta.url), 'utf8'),
    readFile(new URL('../../web-client/src/auth.ts', import.meta.url), 'utf8'),
  ])
  assert.match(api, /\/user\/preferences/)
  assert.match(appStore, /applyUserPreferences/)
  assert.match(appStore, /preference_language/)
  assert.match(userStore, /preference_theme/)
  assert.match(settings, /kessoku:account-theme/)
  assert.match(webClient, /syncAccountPreferences/)
  assert.match(webAuth, /preference_language/)
  assert.match(webAuth, /Partitioned|API preference update/)
})

test('language menus use human names in a stable common-use order', async () => {
  const appStore = await source('src/store/app.js')
  const labels = ['简体中文', '繁体中文', 'English', '日本語', '한국어', 'Français', 'Español', 'Русский']
  let previous = -1
  for (const label of labels) {
    const current = appStore.indexOf(`name: '${label}'`)
    assert.ok(current > previous, `${label} is out of order`)
    previous = current
  }
  assert.doesNotMatch(appStore, /name: '中文'|name: '中文繁体'/)
})
