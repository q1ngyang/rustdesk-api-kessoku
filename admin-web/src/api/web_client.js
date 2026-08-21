import axios from 'axios'
import { getToken } from '@/utils/auth'
import { useUserStore } from '@/store/user'
import { pinia } from '@/store'

const grantEndpoint = '/api/web-client/v1/grants'
const logoutEndpoint = '/api/web-client/v1/logout'

export async function createConnectionGrant () {
  const userStore = useUserStore(pinia)
  const accessToken = userStore.token || getToken()
  if (!accessToken) throw new Error('Authentication is required')

  const response = await axios.post(grantEndpoint, { platform: 'web' }, {
    headers: {
      Authorization: `Bearer ${accessToken}`,
      'Content-Type': 'application/json',
    },
    timeout: 15000,
    withCredentials: false,
  })
  const grant = response.data?.data ?? response.data
  if (grant?.token_type !== 'Bearer'
    || grant?.audience !== 'rustdesk-connect'
    || grant?.scope !== 'connect:initiate'
    || typeof grant?.connection_token !== 'string'
    || grant.connection_token.length === 0
    || grant.connection_token.length > 4096
    || !Number.isSafeInteger(grant?.expires_at)
    || grant.expires_at <= 0) {
    if (typeof grant?.connection_token === 'string' && grant.connection_token.length > 0 && grant.connection_token.length <= 4096) {
      let invalidToken = grant.connection_token
      grant.connection_token = ''
      try {
        await revokeConnectionGrant(invalidToken)
      } catch {
        // A malformed response is already a launch failure; the token is short lived.
      } finally {
        invalidToken = ''
      }
    }
    throw new Error('Invalid web client connection grant')
  }
  return grant
}

export async function revokeConnectionGrant (connectionToken) {
  if (typeof connectionToken !== 'string' || connectionToken.length === 0 || connectionToken.length > 4096) return

  await axios.post(logoutEndpoint, null, {
    headers: {
      Authorization: `Bearer ${connectionToken}`,
    },
    timeout: 5000,
    withCredentials: false,
  })
}
