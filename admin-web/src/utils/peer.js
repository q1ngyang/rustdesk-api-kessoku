import { app } from '@/api/config'
import { createConnectionGrant, revokeConnectionGrant } from '@/api/web_client'
import { ElMessage } from 'element-plus'

const messageType = 'kessoku.web-client.grant.v1'
const readyMessageType = 'kessoku.web-client.ready.v1'
const acceptedMessageType = 'kessoku.web-client.grant-accepted.v1'
const deliveryDeadlineMs = 10000

function exactOrigin (value) {
  const url = new URL(value)
  if (url.protocol !== 'https:' || url.origin !== value || url.username || url.password) {
    throw new Error('Invalid web client public origin')
  }
  return url.origin
}

function deliverGrant (popup, targetOrigin, payload) {
  return new Promise((resolve, reject) => {
    let delivered = false
    let settled = false
    let timer
    const cleanup = (closePopup) => {
      window.clearTimeout(timer)
      window.removeEventListener('message', receiveMessage)
      payload.token = ''
      if (closePopup && !popup.closed) popup.close()
    }
    const fail = (error) => {
      if (settled) return
      settled = true
      cleanup(true)
      reject(error)
    }
    const receiveMessage = (event) => {
      if (event.origin !== targetOrigin || event.source !== popup) return
      if (!delivered && event.data?.type === readyMessageType) {
        try {
          popup.postMessage(payload, targetOrigin)
          delivered = true
        } catch (error) {
          fail(error)
        }
        return
      }
      if (delivered && event.data?.type === acceptedMessageType && !settled) {
        settled = true
        cleanup(false)
        resolve()
      }
    }
    timer = window.setTimeout(() => fail(new Error('Web client grant handoff timed out')), deliveryDeadlineMs)
    try {
      // Register before navigation so a fast client cannot send ready before
      // the authenticated admin has installed its exact-source listener.
      window.addEventListener('message', receiveMessage)
      popup.location.replace(`${targetOrigin}/`)
    } catch (error) {
      fail(error)
    }
  })
}

export const connectByClient = async (id) => {
  const peerId = String(id ?? '').trim()
  if (!peerId || peerId.length > 128) {
    ElMessage.error('Invalid peer ID')
    return
  }

  // Open synchronously so browser popup blocking cannot tempt callers to put
  // credentials in a URL. The blank page receives no grant.
  const popup = window.open('about:blank', '_blank')
  if (!popup) {
    ElMessage.error('Web client popup was blocked')
    return
  }

  let connectionToken = ''
  try {
    const configResponse = await app()
    const appConfig = configResponse?.data
    if (appConfig?.web_client_mode !== 'builtin') {
      throw new Error('Built-in web client is disabled')
    }
    const targetOrigin = exactOrigin(appConfig.web_client_public_origin)
    let grant = await createConnectionGrant()
    connectionToken = grant.connection_token
    const payload = {
      type: messageType,
      peerId,
      token: connectionToken,
      expiresAt: grant.expires_at,
    }
    grant = null

    await deliverGrant(popup, targetOrigin, payload)
    connectionToken = ''
  } catch {
    popup.close()
    let tokenToRevoke = connectionToken
    connectionToken = ''
    if (tokenToRevoke) {
      try {
        await revokeConnectionGrant(tokenToRevoke)
      } catch {
        // The grant still expires shortly; do not obscure the original launch failure.
      } finally {
        tokenToRevoke = ''
      }
    }
    ElMessage.error('Unable to start the built-in Web Client')
  }
}
