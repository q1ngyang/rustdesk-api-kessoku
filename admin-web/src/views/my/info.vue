<template>
  <div class="overview-page">
    <div class="page-heading">
      <div><h2>{{ T('HelloUser', { name: userStore.nickname || userStore.username || T('User') }) }}</h2><p>{{ overviewDescription }}</p></div>
      <div class="page-heading__actions">
        <el-button :icon="Edit" @click="openProfileEditor">{{ T('EditPersonalInformation') }}</el-button>
        <el-button v-if="isSuperAdmin && router.hasRoute('ServerControl')" type="primary" :icon="Connection" @click="go('ServerControl')">{{ T('ServerControl') }}</el-button>
      </div>
    </div>

    <section class="overview-feature-grid">
      <article class="profile-card">
        <DeploymentAsset class="profile-card__brand" variant="logo" width="203" :light-url="branding.brand_logo_light_url" :dark-url="branding.brand_logo_dark_url" alt=""/>
        <div class="profile-card__identity">
		<el-upload class="profile-avatar-upload" :show-file-list="false" accept="image/png,image/jpeg,image/webp" :http-request="queueAvatarCrop">
		  <span class="profile-card__avatar-frame"><span class="profile-card__avatar"><img v-if="userStore.avatar" :src="userStore.avatar" alt=""/><template v-else>{{ initial }}</template></span><span class="profile-card__avatar-edit"><el-icon><EditPen/></el-icon></span></span>
		</el-upload>
          <div class="profile-card__copy"><span>{{ roleLabel }}</span><h3>{{ userStore.username }}</h3><p>{{ userStore.email || T('EmailNotSet') }}</p></div>
          <div class="signed-in-card"><i class="status-dot status-dot--success"></i><div><strong>{{ T('SignedIn') }}</strong><small>{{ T('CurrentSessionActive') }}</small></div></div>
        </div>
        <div class="profile-card__blocks" aria-hidden="true"><i></i><i></i><i></i><i></i></div>
      </article>
      <article class="announcement-card">
        <div class="announcement-card__header"><span class="announcement-card__icon"><el-icon><Promotion/></el-icon></span><div><strong>{{ T('Announcement') }}</strong><small>{{ T('FromAdministrator') }}</small></div></div>
        <div v-if="sanitizedAnnouncement" class="announcement-card__content" v-html="sanitizedAnnouncement"></div>
        <div v-else class="announcement-card__empty">{{ T('NoAnnouncements') }}</div>
        <div class="announcement-card__accent" aria-hidden="true"><i></i><i></i><i></i><i></i></div>
      </article>
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
		<div class="security-row">
		  <span class="security-row__icon"><el-icon><Key/></el-icon></span><div><strong>{{ T('TwoFactorAuthentication') }}</strong><small>{{ twoFactor.enabled ? T('TwoFactorEnabledDescription') : T('TwoFactorDisabledDescription') }}</small></div>
		  <el-button v-if="twoFactor.enabled" type="danger" plain @click="openTwoFactor('disable')">{{ T('Disable') }}</el-button>
		  <el-button v-else type="primary" plain :disabled="!twoFactor.available" @click="openTwoFactor('enable')">{{ T('Enable') }}</el-button>
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
      </div>
    </section>
    <AvatarCropDialog ref="avatarCrop" @cropped="handleAvatarUpload"/>
    <changePwdDialog v-model:visible="changePwdVisible"/>
	<el-dialog v-model="profileDialog" :title="T('EditPersonalInformation')" width="min(420px, calc(100vw - 28px))">
	  <el-form label-position="top"><el-form-item :label="T('Nickname')"><el-input v-model.trim="profile.nickname" maxlength="64"/></el-form-item><el-form-item :label="T('Email')"><el-input v-model.trim="profile.email" type="email" maxlength="254"/></el-form-item></el-form>
	  <template #footer><el-button @click="profileDialog = false">{{ T('Cancel') }}</el-button><el-button type="primary" :loading="profile.saving" @click="saveProfile">{{ T('SaveProfile') }}</el-button></template>
	</el-dialog>
	<el-dialog v-model="twoFactorDialog" :title="T(twoFactorMode === 'enable' ? 'EnableTwoFactor' : 'DisableTwoFactor')" width="min(420px, calc(100vw - 28px))" destroy-on-close>
	  <template v-if="twoFactorMode === 'enable'">
		<el-alert :title="T('TwoFactorSetupNotice')" type="info" :closable="false"/>
		<el-form-item :label="T('AuthenticatorSecret')"><el-input :model-value="twoFactor.secret" readonly><template #append><el-button @click="copySecret">{{ T('Copy') }}</el-button></template></el-input></el-form-item>
		<el-form-item :label="T('OTPAuthURI')"><el-input :model-value="twoFactor.uri" type="textarea" :rows="3" readonly/></el-form-item>
	  </template>
	  <el-form-item :label="T('AuthenticatorCode')"><el-input v-model.trim="twoFactor.code" inputmode="numeric" autocomplete="one-time-code" maxlength="6"/></el-form-item>
	  <template #footer><el-button @click="twoFactorDialog = false">{{ T('Cancel') }}</el-button><el-button :type="twoFactorMode === 'enable' ? 'primary' : 'danger'" :loading="twoFactor.loading" @click="submitTwoFactor">{{ T('Confirm') }}</el-button></template>
	</el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight, ChatDotRound, Collection, Connection, Edit, EditPen, Key, Lock, Monitor, Promotion, Share, Tickets as Shield, User as UserIcon, UserFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import changePwdDialog from '@/components/changePwdDialog.vue'
import AvatarCropDialog from '@/components/avatar/AvatarCropDialog.vue'
import DeploymentAsset from '@/components/brand/DeploymentAsset.vue'
import { useUserStore } from '@/store/user'
import { useAppStore } from '@/store/app'
import { bind, unbind } from '@/api/oauth'
import { beginTwoFactor, confirmTwoFactor, disableTwoFactor, myOauth, twoFactorStatus, updateCurrentProfile, uploadAvatar } from '@/api/user'
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
const profileDialog = ref(false)
const avatarCrop = ref(null)
const profile = reactive({ nickname: '', email: '', saving: false })
const oidcData = ref([])
const twoFactorDialog = ref(false)
const twoFactorMode = ref('enable')
const twoFactor = reactive({ available: false, enabled: false, secret: '', uri: '', code: '', loading: false })
const stats = reactive({ peers: '—', addressBooks: '—', shares: '—', users: '—', groups: '—', loading: true })
const isAdmin = computed(() => userStore.role === 'admin' || userStore.role === 'super_admin')
const isSuperAdmin = computed(() => userStore.role === 'super_admin')
const roleLabel = computed(() => ({ admin: T('ScopedAdministrator'), super_admin: T('SuperAdministrator') }[userStore.role] || T('PersonalWorkspace')))
const initial = computed(() => (userStore.username || 'K').slice(0, 1).toUpperCase())
const branding = computed(() => appStore.setting.branding)
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
const openProfileEditor = () => {
  profile.nickname = userStore.nickname || ''; profile.email = userStore.email || ''; profileDialog.value = true
}
const saveProfile = async () => {
  profile.saving = true
  try {
    const result = await updateCurrentProfile({ nickname: profile.nickname, email: profile.email })
    userStore.$patch(result.data || {})
    profileDialog.value = false
    ElMessage.success(T('ProfileUpdated'))
  } finally { profile.saving = false }
}
const go = name => { if (router.hasRoute(name)) router.push({ name }) }
const getMyOauth = async () => {
  const result = await myOauth().catch(() => false)
  if (result) oidcData.value = result.data || []
}
const loadTwoFactor = async () => Object.assign(twoFactor, (await twoFactorStatus()).data || {})
const openTwoFactor = async mode => {
	 twoFactorMode.value = mode; twoFactor.code = ''; twoFactor.secret = ''; twoFactor.uri = ''
	 if (mode === 'enable') {
	   const reauthentication = await ElMessageBox.prompt(T('TwoFactorReauthentication'), T('EnableTwoFactor'), {
		 confirmButtonText: T('Confirm'), cancelButtonText: T('Cancel'), inputType: 'password', inputAutocomplete: 'current-password', customClass: 'compact-message-box',
	   }).catch(() => null)
	   if (!reauthentication) return
	   const data = (await beginTwoFactor(reauthentication.value || '')).data
	   twoFactor.secret = data.secret; twoFactor.uri = data.otpauth_uri
	 }
	 twoFactorDialog.value = true
}
const copySecret = async () => { await navigator.clipboard.writeText(twoFactor.secret); ElMessage.success(T('Copied')) }
const queueAvatarCrop = ({ file }) => avatarCrop.value?.open(file)
const handleAvatarUpload = async file => {
	try {
	  const result = await uploadAvatar(file)
	  userStore.avatar = result.data.avatar
	  ElMessage.success(T('AvatarUpdated'))
	} catch { ElMessage.error(T('AvatarUploadFailed')) }
}
const submitTwoFactor = async () => {
	if (!/^\d{6}$/.test(twoFactor.code)) { ElMessage.warning(T('EnterAuthenticatorCode')); return }
	twoFactor.loading = true
	try {
	  if (twoFactorMode.value === 'enable') await confirmTwoFactor(twoFactor.code)
	  else await disableTwoFactor(twoFactor.code)
	  ElMessage.success(T('TwoFactorChangedRelogin'))
	  userStore.logout()
	  window.location.reload()
	} finally { twoFactor.loading = false }
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
onMounted(() => { getMyOauth(); loadStats(); loadTwoFactor() })
</script>

<style scoped lang="scss">
.overview-feature-grid { display: grid; align-items: stretch; gap: 14px; margin-bottom: 16px; grid-template-columns: repeat(3, minmax(0, 1fr)); }
.profile-card,.announcement-card { position: relative; min-width: 0; overflow: hidden; border: 1px solid var(--border-subtle); border-radius: 20px; background: var(--surface-1); box-shadow: var(--shadow-sm); }
.profile-card { container-type: inline-size; display: flex; min-height: 176px; align-items: center; padding: 28px 24px 22px; background: linear-gradient(125deg, var(--surface-1), color-mix(in srgb, var(--primary-soft) 48%, var(--surface-1))); }
.announcement-card { grid-column: span 2; }
.profile-card__brand { position: absolute; top: 15px; right: 18px; width: min(203px, 46%) !important; max-width: 203px; height: 48px; object-fit: contain; object-position: right center; }
.profile-card__identity { position: relative; z-index: 2; display: grid; width: 100%; min-width: 0; align-items: center; gap: 0 15px; grid-template-columns: 68px minmax(118px, 1fr) minmax(136px, .76fr); padding-top: 40px; }
.profile-avatar-upload { grid-column: 1; grid-row: 1; }.profile-card__avatar-frame { position: relative; display: block; width: 68px; height: 68px; cursor: pointer; }.profile-card__avatar { position: absolute; inset: 0; display: grid; width: 68px; height: 68px; overflow: hidden; place-items: center; border-radius: 20px; background: linear-gradient(145deg, var(--primary), #8564ed); color: #fff; font-size: 21px; font-weight: 800; box-shadow: 0 12px 28px rgba(94,115,238,.28); }.profile-card__avatar img { width: 100%; height: 100%; object-fit: cover; }.profile-card__avatar-edit { position: absolute; right: -6px; bottom: -6px; z-index: 3; display: grid; width: 27px; height: 27px; place-items: center; border: 3px solid var(--surface-1); border-radius: 50%; background: var(--primary); color: #fff; font-size: 12px; box-shadow: var(--shadow-sm); }
.profile-card__copy { min-width: 0; }.profile-card__copy span { color: var(--primary); font-size: 10px; font-weight: 800; letter-spacing: .08em; text-transform: uppercase; }.profile-card__copy h3 { margin: 4px 0 2px; color: var(--text-primary); font-size: 22px; letter-spacing: -.03em; }.profile-card__copy p { margin: 0; overflow: hidden; color: var(--text-tertiary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.signed-in-card { display: flex; width: fit-content; min-width: 148px; align-items: center; justify-self: end; gap: 10px; margin-top: 0; padding: 11px 14px; border: 1px solid color-mix(in srgb, var(--success) 23%, var(--border-subtle)); border-radius: 15px; background: color-mix(in srgb, var(--success-soft) 60%, var(--surface-1)); }.signed-in-card .status-dot { width: 11px; height: 11px; }.signed-in-card > div { display: flex; flex-direction: column; }.signed-in-card strong { color: var(--text-primary); font-size: 12px; }.signed-in-card small { margin-top: 2px; color: var(--text-tertiary); font-size: 9px; }
.profile-card__blocks { position: absolute; bottom: -4px; left: 24px; display: flex; gap: 4px; }.profile-card__blocks i { width: 23px; height: 7px; border-radius: 3px 3px 0 0; }.profile-card__blocks i:nth-child(1){background:var(--kessoku-red)}.profile-card__blocks i:nth-child(2){background:var(--kessoku-yellow)}.profile-card__blocks i:nth-child(3){background:var(--kessoku-blue)}.profile-card__blocks i:nth-child(4){background:var(--kessoku-pink)}
.announcement-card { display: grid; min-height: 176px; grid-template-columns: 116px minmax(0, 1fr); gap: 20px; padding: 22px 22px 20px; }.announcement-card__header { display: flex; min-height: 100%; align-items: center; justify-content: center; flex-direction: column; gap: 10px; padding-right: 18px; border-right: 1px solid var(--border-subtle); text-align: center; }.announcement-card__icon { display: grid; width: 40px; height: 40px; place-items: center; border-radius: 13px; background: color-mix(in srgb, var(--kessoku-red) 13%, var(--surface-1)); color: #c5515d; font-size: 18px; }.announcement-card__header > div { display: flex; align-items: center; flex-direction: column; }.announcement-card__header strong { color: var(--text-primary); font-size: 14px; }.announcement-card__header small { max-width: 94px; margin-top: 4px; color: var(--text-tertiary); font-size: 9px; line-height: 1.4; }.announcement-card__content,.announcement-card__empty { max-height: 132px; align-self: center; overflow: auto; color: var(--text-secondary); font-size: 12px; line-height: 1.7; }.announcement-card__empty { color: var(--text-tertiary); }.announcement-card__content :deep(p),.announcement-card__content :deep(h1),.announcement-card__content :deep(h2),.announcement-card__content :deep(h3) { margin: 0 0 5px; }.announcement-card__content :deep(img) { max-width: 100%; }.announcement-card__accent { position: absolute; top: 0; right: 18px; display: flex; gap: 4px; }.announcement-card__accent i { width: 13px; height: 5px; border-radius: 0 0 3px 3px; }.announcement-card__accent i:nth-child(1){background:var(--kessoku-red)}.announcement-card__accent i:nth-child(2){background:var(--kessoku-yellow)}.announcement-card__accent i:nth-child(3){background:var(--kessoku-blue)}.announcement-card__accent i:nth-child(4){background:var(--kessoku-pink)}
.metric-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; margin-bottom: 16px; }.metric-card { display: flex; min-width: 0; align-items: center; gap: 13px; padding: 17px; border: 1px solid var(--border-subtle); border-radius: 17px; background: var(--surface-1); box-shadow: var(--shadow-sm); color: inherit; cursor: pointer; text-align: left; transition: transform var(--motion-fast), box-shadow var(--motion-fast), border-color var(--motion-fast); }.metric-card:hover { border-color: var(--primary-border); box-shadow: var(--shadow-md); transform: translateY(-2px); }
.metric-card__icon { display: grid; width: 42px; height: 42px; flex: 0 0 auto; place-items: center; border-radius: 13px; color: var(--primary); font-size: 18px; }.metric-card__icon--blue { background: var(--primary-soft); }.metric-card__icon--yellow { background: color-mix(in srgb, var(--kessoku-yellow) 18%, var(--surface-1)); color: #b08718; }.metric-card__icon--pink { background: color-mix(in srgb, var(--kessoku-pink) 16%, var(--surface-1)); color: #c05b84; }
.metric-card__icon--red { background: color-mix(in srgb, var(--kessoku-red) 15%, var(--surface-1)); color: #c5525d; }
.metric-card__copy { display: flex; min-width: 0; flex: 1; flex-direction: column; }.metric-card__copy small { color: var(--text-tertiary); font-size: 11px; }.metric-card__copy strong { margin-top: 3px; color: var(--text-primary); font-size: 21px; }.metric-card__arrow { color: var(--text-tertiary); }
.overview-grid { display: grid; align-items: start; grid-template-columns: minmax(0, 1.35fr) minmax(300px, .65fr); gap: 16px; }.overview-side { display: grid; gap: 16px; }.card-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }.card-header h3 { margin: 0; color: var(--text-primary); font-size: 16px; letter-spacing: -.02em; }.card-header p { margin: 4px 0 0; color: var(--text-tertiary); font-size: 11px; }.card-header__icon { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 11px; background: var(--success-soft); color: var(--success); }
.security-row { display: flex; align-items: center; gap: 12px; padding: 14px 0; border-bottom: 1px solid var(--border-subtle); }.security-row:last-child { border-bottom: 0; }.security-row__icon { display: grid; width: 36px; height: 36px; flex: 0 0 auto; place-items: center; border-radius: 11px; background: var(--surface-3); color: var(--text-secondary); }.security-row > div { display: flex; min-width: 0; flex: 1; flex-direction: column; }.security-row strong { color: var(--text-primary); font-size: 12px; }.security-row small { margin-top: 3px; overflow: hidden; color: var(--text-tertiary); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }.security-empty { padding: 22px 0; color: var(--text-tertiary); font-size: 12px; text-align: center; }
.quick-card button { display: flex; width: 100%; align-items: center; gap: 11px; padding: 11px 8px; border: 0; border-radius: 11px; background: transparent; color: inherit; cursor: pointer; text-align: left; }.quick-card button:hover { background: var(--surface-2); }.quick-card button > span { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 10px; background: var(--primary-soft); color: var(--primary); }.quick-card button > div { display: flex; min-width: 0; flex: 1; flex-direction: column; }.quick-card button strong { color: var(--text-primary); font-size: 12px; }.quick-card button small { margin-top: 2px; color: var(--text-tertiary); font-size: 10px; }.quick-card button > .el-icon { color: var(--text-tertiary); }
.quick-card button > .quick-card__icon--yellow { background: color-mix(in srgb, var(--kessoku-yellow) 17%, var(--surface-1)); color: #a97d0c; }.quick-card button > .quick-card__icon--pink { background: color-mix(in srgb, var(--kessoku-pink) 15%, var(--surface-1)); color: #bd5d84; }
@media (max-width: 1000px) { .overview-grid { grid-template-columns: 1fr; }.overview-side { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@container (max-width: 400px) { .profile-card__identity { gap: 12px 14px; grid-template-columns: 68px minmax(0, 1fr); }.signed-in-card { grid-column: 1 / -1; justify-self: center; margin-top: 4px; } }
@media (max-width: 980px) { .overview-feature-grid { grid-template-columns: 1fr; }.announcement-card { grid-column: auto; min-height: 150px; grid-template-columns: 116px minmax(0, 1fr); } }
@media (max-width: 650px) { .profile-card { min-height: 224px; padding: 22px 18px; }.profile-card__brand { top: 13px; right: 16px; width: min(142px, 48%) !important; height: 36px; }.profile-card__identity { gap: 12px 14px; grid-template-columns: 62px minmax(0, 1fr); padding-top: 38px; }.profile-card__avatar-frame,.profile-card__avatar { width: 62px; height: 62px; }.signed-in-card { grid-column: 1 / -1; justify-self: center; min-width: 154px; margin-top: 6px; }.announcement-card { display: flex; min-height: 0; flex-direction: column; gap: 14px; padding: 18px; }.announcement-card__header { min-height: auto; justify-content: center; padding: 0 0 13px; border-right: 0; border-bottom: 1px solid var(--border-subtle); }.announcement-card__header small { max-width: none; }.announcement-card__content,.announcement-card__empty { width: 100%; max-height: 140px; }.metric-grid { grid-template-columns: 1fr; gap: 10px; }.metric-card { padding: 14px; }.overview-side { grid-template-columns: 1fr; }.security-row { align-items: flex-start; }.security-row .el-button { flex: 0 0 auto; } }
</style>
