const UUID_KEY = 'kessoku-admin-device-uuid'

function detectedBrowser () {
  const ua = navigator.userAgent || ''
  if (/edg/i.test(ua)) return 'Edge'
  if (/chrome|crios/i.test(ua)) return 'Chrome'
  if (/firefox|fxios/i.test(ua)) return 'Firefox'
  if (/safari/i.test(ua)) return 'Safari'
  return 'Web browser'
}

export function detectedPlatform () {
  const platform = navigator.userAgentData?.platform || navigator.platform || 'Web'
  if (/mac/i.test(platform)) return 'macOS'
  if (/win/i.test(platform)) return 'Windows'
  if (/android|linux arm/i.test(platform)) return 'Android'
  if (/linux/i.test(platform)) return 'Linux'
  if (/iphone|ipad|ios/i.test(platform)) return 'iOS'
  return String(platform).slice(0, 64)
}

export function browserDeviceIdentity () {
  let uuid = localStorage.getItem(UUID_KEY) || ''
  if (!/^[0-9a-f-]{36}$/i.test(uuid)) {
    uuid = crypto.randomUUID()
    localStorage.setItem(UUID_KEY, uuid)
  }
  const platform = detectedPlatform()
  return {
    device_id: `${detectedBrowser()} · ${platform}`.slice(0, 128),
    uuid,
    platform,
  }
}
