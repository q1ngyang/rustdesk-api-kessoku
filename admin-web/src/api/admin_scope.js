import request from '@/utils/request'

export function detail (userId) {
  return request({ url: `/admin_scope/detail/${userId}` })
}

export function options (params) {
  return request({ url: '/admin_scope/options', params })
}

export function update (data) {
  return request({ url: '/admin_scope/update', method: 'post', data })
}
