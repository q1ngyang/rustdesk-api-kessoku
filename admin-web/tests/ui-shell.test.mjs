import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const source = path => readFile(new URL(`../${path}`, import.meta.url), 'utf8')

test('v3 shell keeps every permission-filtered route reachable on small screens', async () => {
  const [layout, bottomNav, menu] = await Promise.all([
    source('src/layout/index.vue'),
    source('src/layout/components/MobileBottomNav.vue'),
    source('src/layout/components/menu/index.vue'),
  ])
  assert.match(layout, /mobileSidebarOpen/)
  assert.match(layout, /<g-aside/)
  assert.match(layout, /<mobile-bottom-nav/)
  assert.match(bottomNav, /routeStore\.routes/)
  assert.match(bottomNav, /openMobileSidebar/)
  assert.match(menu, /routeStore\.routes/)
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
  const [tokens, brand, themedAsset, themeAssets, auth, authAction, header, overview, login, serverControl, about] = await Promise.all([
    source('src/styles/style.scss'),
    source('src/components/brand/KessokuBrand.vue'),
    source('src/components/brand/ThemeBrandAsset.vue'),
    source('src/utils/themeAssets.js'),
    source('src/components/auth/AuthLayout.vue'),
    source('src/components/auth/AuthActionLayout.vue'),
    source('src/layout/components/header.vue'),
    source('src/views/my/info.vue'),
    source('src/views/login/login.vue'),
    source('src/views/server_control/index.vue'),
    source('src/views/about/index.vue'),
  ])
  for (const color of ['blue', 'yellow', 'red', 'pink']) assert.match(tokens, new RegExp(`--kessoku-${color}:`))
  assert.match(brand, /<StarryIcon/)
  assert.match(brand, /RustDesk API/)
  assert.match(brand, /KESSOKU/)
  assert.match(themedAsset, /starrylinks-icon-light\.svg/)
  assert.match(themedAsset, /starrylinks-icon-dark\.svg/)
  assert.match(themedAsset, /html\.dark/)
  assert.match(themeAssets, /dark \? iconDark : iconLight/)
  assert.doesNotMatch(auth, /auth-story__palette/)
  assert.doesNotMatch(authAction, /action-panel__palette/)
  assert.doesNotMatch(header, /topbar__accent/)
  assert.match(overview, /quick-card__icon--pink/)
  assert.match(overview, /quick-card__icon--yellow/)
  assert.match(login, /show-starry-logo/)
  assert.match(serverControl, /<StarryLogo/)
  assert.match(about, /<StarryLogo/)
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
