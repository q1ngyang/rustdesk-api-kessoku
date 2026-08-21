import axios from 'axios'
import { ElMessage } from 'element-plus'
import { getToken } from '@/utils/auth'
import { useUserStore } from '@/store/user'
import { useAppStore } from '@/store/app'
import { pinia } from '@/store'

const controlRequest = axios.create({
  baseURL: import.meta.env.VITE_SERVER_API,
  withCredentials: true,
  timeout: 30000,
  maxContentLength: 2 * 1024 * 1024,
})

controlRequest.interceptors.request.use(config => {
  const userStore = useUserStore(pinia)
  const appStore = useAppStore(pinia)
  const token = userStore.token || getToken()

  config.headers = config.headers || {}
  if (token) {
    config.headers['api-token'] = token
  }
  if (appStore.setting.lang) {
    config.headers['Accept-Language'] = appStore.setting.lang
  }
  if (globalThis.crypto?.randomUUID) {
    config.headers['X-Request-ID'] = globalThis.crypto.randomUUID()
  }
  return config
})

controlRequest.interceptors.response.use(response => {
  const body = response.data
  if (!body || typeof body !== 'object' || !Object.prototype.hasOwnProperty.call(body, 'data')) {
    return Promise.reject(new Error('Invalid server-control response envelope'))
  }
  return body
}, error => {
  const body = error.response?.data
  const problem = body?.error
  const code = problem?.code || body?.code || `HTTP_${error.response?.status || 'ERROR'}`
  const message = problem?.message || body?.message || error.message || 'Server-control request failed'
  const requestID = problem?.request_id
  ElMessage({
    message: requestID ? `${code}: ${message} (${requestID})` : `${code}: ${message}`,
    type: 'error',
    duration: 7000,
  })
  return Promise.reject({ code, message, request_id: requestID, retryable: problem?.retryable === true })
})

export default controlRequest
