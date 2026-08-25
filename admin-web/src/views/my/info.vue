<template>
  <div class="overview-page">
    <div class="page-heading">
      <div><h2>{{ T('HelloUser', { name: userStore.nickname || userStore.username || T('User') }) }}</h2><p>{{ overviewDescription }}</p></div>
      <div class="page-heading__actions">
        <el-button :icon="Lock" @click="showChangePwd">{{ T('ChangePassword') }}</el-button>
        <el-button v-if="isSuperAdmin && router.hasRoute('ServerControl')" type="primary" :icon="Connection" @click="go('ServerControl')">{{ T('ServerControl') }}</el-button>
      </div>
    </div>

    <section class="profile-hero">
      <div class="profile-hero__identity">
        <span class="profile-hero__avatar">{{ initial }}</span>
        <div><span class="profile-hero__eyebrow">{{ roleLabel }}</span><h3>{{ userStore.username }}</h3><p>{{ userStore.email || T('EmailNotSet') }}</p></div>
      </div>
      <div class="profile-hero__meta"><span><i class="status-dot status-dot--success"></i>{{ T('SignedIn') }}</span><small>{{ T('AccountProtected') }}</small></div>
      <div class="profile-hero__shapes" aria-hidden="true"><i></i><i></i></div>
    </section>

    <section class="metric-grid" aria-label="Personal overview">
      <button v-for="metric in metrics" :key="metric.route" class="metric-card" type="button" @click="go(metric.route)">
        <span class="metric-card__icon" :class="`metric-card__icon--${metric.tone}`"><el-icon><component :is="metric.icon"/></el-icon></span>
        <span class="metric-card__copy"><small>{{ T(metric.label) }}</small><strong>{{ stats.loading ? '—' : metric.value }}</strong></span>
        <el-icon class="metric-card__arrow"><ArrowRight/></el-icon>
      </button>
    </section>

    <section class="overview-grid">
      <el-card class="security-card" shadow="never">
        <template #header><div class="card-header"><div><h3>{{ T('AccountAndSecurity') }}</h3><p>{{ T('ManageSignInMethods') }}</p></div><span class="card-header__icon"><el-icon><Shield/></el-icon></span></div></template>
        <div class="security-row">
          <span class="security-row__icon"><el-icon><Lock/></el-icon></span><div><strong>{{ T('Password') }}</strong><small>{{ T('PasswordSecurityDescription') }}</small></div><el-button plain @click="showChangePwd">{{ T('Change') }}</el-button>
        </div>
        <div v-for="item in oidcData" :key="item.op" class="security-row">
          <span class="security-row__icon"><el-icon><Key/></el-icon></span><div><strong>{{ item.op }}</strong><small>{{ item.status === 1 ? T('IdentityProviderConnected') : T('IdentityProviderNotConnected') }}</small></div>
          <el-button v-if="item.status === 1" type="danger" plain @click="toUnBind(item)">{{ T('UnBind') }}</el-button>
          <el-button v-else type="primary" plain @click="toBind(item)">{{ T('ToBind') }}</el-button>
        </div>
        <div v-if="!oidcData.length" class="security-empty">{{ T('NoIdentityProviders') }}</div>
      </el-card>

      <div class="overview-side">
        <el-card class="quick-card" shadow="never">
          <template #header><div class="card-header"><div><h3>{{ T('QuickAccess') }}</h3><p>{{ T('CommonTasksDescription') }}</p></div></div></template>
          <button v-for="link in quickLinks" :key="link.route" type="button" @click="go(link.route)">
            <span :class="`quick-card__icon--${link.tone}`"><el-icon><component :is="link.icon"/></el-icon></span><div><strong>{{ T(link.label) }}</strong><small>{{ T(link.description) }}</small></div><el-icon><ArrowRight/></el-icon>
          </button>
        </el-card>
        <el-card v-if="sanitizedAnnouncement" class="announcement-card" shadow="never">
          <template #header><div class="card-header"><div><h3>{{ T('Announcement') }}</h3><p>{{ T('FromAdministrator') }}</p></div></div></template>
          <div class="announcement-card__content" v-html="sanitizedAnnouncement"></div>
        </el-card>
      </div>
    </section>
    <changePwdDialog v-model:visible="changePwdVisible"/>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight, ChatDotRound, Collection, Connection, Key, Lock, Monitor, Share, Tickets as Shield, User as UserIcon, UserFilled } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import changePwdDialog from '@/components/changePwdDialog.vue'
import { useUserStore } from '@/store/user'
import { useAppStore } from '@/store/app'
import { bind, unbind } from '@/api/oauth'
import { myOauth } from '@/api/user'
import { list as peerList } from '@/api/my/peer'
import { list as addressBookList } from '@/api/my/address_book'
import { list as shareRecordList } from '@/api/my/share_record'
import { list as adminPeerList } from '@/api/peer'
import { list as adminUserList } from '@/api/user'
import { list as adminGroupList } from '@/api/group'
import { T } from '@/utils/i18n'

const appStore = useAppStore()
const userStore = useUserStore()
const router = useRouter()
const changePwdVisible = ref(false)
const oidcData = ref([])
const stats = reactive({ peers: '—', addressBooks: '—', shares: '—', users: '—', groups: '—', loading: true })
const isAdmin = computed(() => userStore.role === 'admin' || userStore.role === 'super_admin')
const isSuperAdmin = computed(() => userStore.role === 'super_admin')
const roleLabel = computed(() => ({ admin: T('ScopedAdministrator'), super_admin: T('SuperAdministrator') }[userStore.role] || T('PersonalWorkspace')))
const initial = computed(() => (userStore.username || 'K').slice(0, 1).toUpperCase())
const overviewDescription = computed(() => T(isAdmin.value ? 'AdminOverviewDescription' : 'OverviewDescription'))
const sanitizedAnnouncement = computed(() => {
  const rendered = marked.parse(appStore.setting.hello || '')
  return DOMPurify.sanitize(rendered, { USE_PROFILES: { html: true } })
})
const metrics = computed(() => isAdmin.value ? [
  { label: 'UserManage', value: stats.users, route: 'UserList', icon: UserFilled, tone: 'blue' },
  { label: 'PeerManage', value: stats.peers, route: 'Peer', icon: Monitor, tone: 'yellow' },
  { label: 'GroupManage', value: stats.groups, route: 'UserGroup', icon: ChatDotRound, tone: 'red' },
] : [
  { label: 'Devices', value: stats.peers, route: 'MyPeer', icon: Monitor, tone: 'blue' },
  { label: 'AddressBooks', value: stats.addressBooks, route: 'MyAddressBookList', icon: Collection, tone: 'yellow' },
  { label: 'ShareRecord', value: stats.shares, route: 'MyShareRecordList', icon: Share, tone: 'pink' },
])
const quickLinks = computed(() => (isAdmin.value ? [
  { route: 'Peer', label: 'ManageDevices', description: 'AllDevicesDescription', icon: Monitor, tone: 'blue' },
  { route: 'UserList', label: 'UserManage', description: 'AllUsersDescription', icon: UserFilled, tone: 'pink' },
  { route: 'ServerControl', label: 'ServerControl', description: 'ServerControlDescription', icon: Connection, tone: 'yellow' },
] : [
  { route: 'MyPeer', label: 'ManageDevices', description: 'ManageDevicesDescription', icon: Monitor, tone: 'blue' },
  { route: 'MyAddressBookList', label: 'OpenAddressBook', description: 'OpenAddressBookDescription', icon: Collection, tone: 'yellow' },
  { route: 'MyLoginLog', label: 'ReviewLoginActivity', description: 'ReviewLoginActivityDescription', icon: UserIcon, tone: 'pink' },
]).filter(link => router.hasRoute(link.route)))

const showChangePwd = () => { changePwdVisible.value = true }
const go = name => { if (router.hasRoute(name)) router.push({ name }) }
const getMyOauth = async () => {
  const result = await myOauth().catch(() => false)
  if (result) oidcData.value = result.data || []
}
const toBind = async row => {
  const result = await bind({ op: row.op }).catch(() => false)
  if (result?.data?.url) {
    const popup = window.open(result.data.url, '_blank', 'noopener,noreferrer')
    if (popup) popup.opener = null
  }
}
const toUnBind = async row => {
  const confirmed = await ElMessageBox.confirm(T('Confirm?', { param: T('UnBind') }), { confirmButtonText: T('Confirm'), cancelButtonText: T('Cancel'), type: 'warning' }).catch(() => false)
  if (confirmed && await unbind({ op: row.op }).catch(() => false)) await getMyOauth()
}
const loadStats = async () => {
  const query = { page: 1, page_size: 1 }
  if (isAdmin.value) {
    const [users, peers, groups] = await Promise.allSettled([adminUserList(query), adminPeerList(query), adminGroupList(query)])
    stats.users = users.status === 'fulfilled' ? users.value.data.total ?? users.value.data.list?.length ?? 0 : '—'
    stats.peers = peers.status === 'fulfilled' ? peers.value.data.total ?? peers.value.data.list?.length ?? 0 : '—'
    stats.groups = groups.status === 'fulfilled' ? groups.value.data.total ?? groups.value.data.list?.length ?? 0 : '—'
    stats.loading = false
    return
  }
  const [peers, addressBooks, shares] = await Promise.allSettled([peerList(query), addressBookList(query), shareRecordList(query)])
  stats.peers = peers.status === 'fulfilled' ? peers.value.data.total ?? peers.value.data.list?.length ?? 0 : '—'
  stats.addressBooks = addressBooks.status === 'fulfilled' ? addressBooks.value.data.total ?? addressBooks.value.data.list?.length ?? 0 : '—'
  stats.shares = shares.status === 'fulfilled' ? shares.value.data.total ?? shares.value.data.list?.length ?? 0 : '—'
  stats.loading = false
}
onMounted(() => { getMyOauth(); loadStats() })
</script>

<style scoped lang="scss">
.profile-hero { position: relative; display: flex; min-height: 142px; align-items: center; justify-content: space-between; gap: 24px; overflow: hidden; margin-bottom: 16px; padding: 26px 30px; border: 1px solid var(--primary-border); border-radius: 20px; background: linear-gradient(125deg, var(--surface-1), color-mix(in srgb, var(--primary-soft) 58%, var(--surface-1))); box-shadow: var(--shadow-sm); }
.profile-hero__identity { position: relative; z-index: 2; display: flex; align-items: center; gap: 17px; }.profile-hero__avatar { display: grid; width: 62px; height: 62px; flex: 0 0 auto; place-items: center; border-radius: 19px; background: linear-gradient(145deg, var(--primary), #8564ed); color: #fff; font-size: 22px; font-weight: 800; box-shadow: 0 12px 28px rgba(94, 115, 238, .28); }
.profile-hero__eyebrow { color: var(--primary); font-size: 10px; font-weight: 800; letter-spacing: .08em; text-transform: uppercase; }.profile-hero h3 { margin: 4px 0 2px; color: var(--text-primary); font-size: 22px; letter-spacing: -.03em; }.profile-hero p { margin: 0; color: var(--text-tertiary); font-size: 12px; }
.profile-hero__meta { position: relative; z-index: 2; display: flex; align-items: flex-end; gap: 5px; color: var(--text-secondary); font-size: 12px; flex-direction: column; }.profile-hero__meta small { color: var(--text-tertiary); }
.profile-hero__shapes i { position: absolute; left: -6px; width: 22px; height: 22px; border-radius: 7px; opacity: .85; }.profile-hero__shapes i:first-child { top: 22px; background: var(--kessoku-blue); transform: rotate(5deg); }.profile-hero__shapes i:last-child { top: 45px; background: var(--kessoku-yellow); transform: rotate(-4deg); }
.metric-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; margin-bottom: 16px; }.metric-card { display: flex; min-width: 0; align-items: center; gap: 13px; padding: 17px; border: 1px solid var(--border-subtle); border-radius: 17px; background: var(--surface-1); box-shadow: var(--shadow-sm); color: inherit; cursor: pointer; text-align: left; transition: transform var(--motion-fast), box-shadow var(--motion-fast), border-color var(--motion-fast); }.metric-card:hover { border-color: var(--primary-border); box-shadow: var(--shadow-md); transform: translateY(-2px); }
.metric-card__icon { display: grid; width: 42px; height: 42px; flex: 0 0 auto; place-items: center; border-radius: 13px; color: var(--primary); font-size: 18px; }.metric-card__icon--blue { background: var(--primary-soft); }.metric-card__icon--yellow { background: color-mix(in srgb, var(--kessoku-yellow) 18%, var(--surface-1)); color: #b08718; }.metric-card__icon--pink { background: color-mix(in srgb, var(--kessoku-pink) 16%, var(--surface-1)); color: #c05b84; }
.metric-card__icon--red { background: color-mix(in srgb, var(--kessoku-red) 15%, var(--surface-1)); color: #c5525d; }
.metric-card__copy { display: flex; min-width: 0; flex: 1; flex-direction: column; }.metric-card__copy small { color: var(--text-tertiary); font-size: 11px; }.metric-card__copy strong { margin-top: 3px; color: var(--text-primary); font-size: 21px; }.metric-card__arrow { color: var(--text-tertiary); }
.overview-grid { display: grid; align-items: start; grid-template-columns: minmax(0, 1.35fr) minmax(300px, .65fr); gap: 16px; }.overview-side { display: grid; gap: 16px; }.card-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }.card-header h3 { margin: 0; color: var(--text-primary); font-size: 16px; letter-spacing: -.02em; }.card-header p { margin: 4px 0 0; color: var(--text-tertiary); font-size: 11px; }.card-header__icon { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 11px; background: var(--success-soft); color: var(--success); }
.security-row { display: flex; align-items: center; gap: 12px; padding: 14px 0; border-bottom: 1px solid var(--border-subtle); }.security-row:last-child { border-bottom: 0; }.security-row__icon { display: grid; width: 36px; height: 36px; flex: 0 0 auto; place-items: center; border-radius: 11px; background: var(--surface-3); color: var(--text-secondary); }.security-row > div { display: flex; min-width: 0; flex: 1; flex-direction: column; }.security-row strong { color: var(--text-primary); font-size: 12px; }.security-row small { margin-top: 3px; overflow: hidden; color: var(--text-tertiary); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }.security-empty { padding: 22px 0; color: var(--text-tertiary); font-size: 12px; text-align: center; }
.quick-card button { display: flex; width: 100%; align-items: center; gap: 11px; padding: 11px 8px; border: 0; border-radius: 11px; background: transparent; color: inherit; cursor: pointer; text-align: left; }.quick-card button:hover { background: var(--surface-2); }.quick-card button > span { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 10px; background: var(--primary-soft); color: var(--primary); }.quick-card button > div { display: flex; min-width: 0; flex: 1; flex-direction: column; }.quick-card button strong { color: var(--text-primary); font-size: 12px; }.quick-card button small { margin-top: 2px; color: var(--text-tertiary); font-size: 10px; }.quick-card button > .el-icon { color: var(--text-tertiary); }
.quick-card button > .quick-card__icon--yellow { background: color-mix(in srgb, var(--kessoku-yellow) 17%, var(--surface-1)); color: #a97d0c; }.quick-card button > .quick-card__icon--pink { background: color-mix(in srgb, var(--kessoku-pink) 15%, var(--surface-1)); color: #bd5d84; }
.announcement-card__content { overflow: hidden; color: var(--text-secondary); font-size: 12px; line-height: 1.7; }.announcement-card__content :deep(img) { max-width: 100%; }
@media (max-width: 1000px) { .overview-grid { grid-template-columns: 1fr; }.overview-side { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 650px) { .profile-hero { min-height: 0; align-items: flex-start; padding: 21px 18px; flex-direction: column; }.profile-hero__meta { align-items: flex-start; }.metric-grid { grid-template-columns: 1fr; gap: 10px; }.metric-card { padding: 14px; }.overview-side { grid-template-columns: 1fr; }.security-row { align-items: flex-start; }.security-row .el-button { flex: 0 0 auto; } }
</style>
