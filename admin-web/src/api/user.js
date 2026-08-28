import request from '@/utils/request'

export function login (data) {
  return request({
    url: '/login',
    method: 'post',
    data,
  })
}

export function current () {
  return request({
    url: '/user/current',
    method: 'get',
  })
}

export function list (params) {
  return request({
    url: '/user/list',
    params,
  })
}

export function detail (id) {
  return request({
    url: `/user/detail/${id}`,
  })
}

export function create (data) {
  return request({
    url: '/user/create',
    method: 'post',
    data,
  })
}

export function update (data) {
  return request({
    url: '/user/update',
    method: 'post',
    data,
  })
}

export function remove (data) {
  return request({
    url: '/user/delete',
    method: 'post',
    data,
  })
}

export function changePwd (data) {
  return request({
    url: '/user/changePwd',
    method: 'post',
    data,
  })
}

export function revokeSessions (data) {
  return request({
    url: '/user/revokeSessions',
    method: 'post',
    data,
  })
}

export function changeCurPwd (data) {
  return request({
    url: '/user/changeCurPwd',
    method: 'post',
    data,
  })
}

export function updateCurrentProfile (data) {
  return request({ url: '/user/profile', method: 'post', data })
}

export function updatePreferences (data) {
  return request({ url: '/user/preferences', method: 'post', data })
}

export function myOauth () {
  return request({
    url: '/user/myOauth',
    method: 'post',
  })
}

export function myPeer (params) {
  return request({
    url: '/user/myPeer',
    params,
  })
}

export function groupUsers (data) {
  return request({
    url: '/user/groupUsers',
    method: 'post',
    data,
  })
}

export function register (data) {
  return request({
    url: '/user/register',
    method: 'post',
    data,
  })
}

export const twoFactorStatus = () => request({ url: '/user/two-factor' })
export const beginTwoFactor = password => request({ url: '/user/two-factor/begin', method: 'post', data: { password } })
export const confirmTwoFactor = code => request({ url: '/user/two-factor/confirm', method: 'post', data: { code } })
export const disableTwoFactor = code => request({ url: '/user/two-factor/disable', method: 'post', data: { code } })

export function uploadAvatar (file) {
  const data = new FormData()
  const upload = file instanceof File ? file : new File([file], 'avatar.jpg', { type: file?.type || 'image/jpeg' })
  data.append('file', upload, upload.name || 'avatar.jpg')
  return request({ url: '/user/avatar', method: 'post', data })
}
