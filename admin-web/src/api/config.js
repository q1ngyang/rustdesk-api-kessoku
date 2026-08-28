import request from '@/utils/request'

export function app () {
  return request({
    url: '/config/app',
    method: 'get',
  })
}

export function admin () {
  return request({
    url: '/config/admin',
    method: 'get',
  })
}

export function branding () {
  return request({ url: '/config/branding' })
}

export function updateBranding (data) {
  return request({ url: '/config/branding', method: 'post', data })
}

export function uploadBrandAsset (file) {
  const data = new FormData()
  const upload = file instanceof File ? file : new File([file], 'brand-image.png', { type: file?.type || 'image/png' })
  data.append('file', upload, upload.name || 'brand-image.png')
  return request({ url: '/config/branding/upload', method: 'post', data })
}

export function about () {
  return request({ url: '/config/about' })
}

export function systemSettings () {
  return request({ url: '/config/system-settings' })
}

export function updateSystemSettings (data) {
  return request({ url: '/config/system-settings', method: 'post', data })
}

export function updateGeoIPDatabase () {
  return request({ url: '/config/system-settings/geoip/update', method: 'post' })
}

export function lookupIP (ip) {
  return request({ url: '/config/geoip/lookup', params: { ip } })
}
