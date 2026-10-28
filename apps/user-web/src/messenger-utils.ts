// WebRTC, E2EE, and Offline Queue utilities for LINE messenger web client

export async function startWebRTCCall(convId: string, kind: 'voice' | 'video', token: string, requestFn: any): Promise<string | null> {
  try {
    const constraints: MediaStreamConstraints = { audio: true, video: kind === 'video' }
    const stream = await navigator.mediaDevices.getUserMedia(constraints)
    const pc = new RTCPeerConnection({ iceServers: [{ urls: 'stun:stun.l.google.com:19302' }] })
    stream.getTracks().forEach(track => pc.addTrack(track, stream))
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)
    const res = await requestFn(`/api/v1/calls`, token, 'POST', { conversation_id: convId, kind, sdp_offer: offer.sdp })
    return res.id
  } catch (e: any) {
    console.error('Call failed:', e)
    return null
  }
}

export function endWebRTCCall(pc: RTCPeerConnection | null, stream: MediaStream | null) {
  if (pc) pc.close()
  if (stream) stream.getTracks().forEach(t => t.stop())
}

export async function generateE2EEKeyPair(): Promise<CryptoKeyPair> {
  return await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, true, ['deriveKey', 'deriveBits'])
}

export async function deriveSharedSecret(privateKey: CryptoKey, publicKey: CryptoKey): Promise<CryptoKey> {
  return await crypto.subtle.deriveKey({ name: 'ECDH', public: publicKey }, privateKey, { name: 'AES-GCM', length: 256 }, false, ['encrypt', 'decrypt'])
}

export async function encryptMessage(plaintext: string, sharedKey: CryptoKey): Promise<string> {
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const encoded = new TextEncoder().encode(plaintext)
  const ct = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, sharedKey, encoded)
  const combined = new Uint8Array(iv.length + ct.byteLength)
  combined.set(iv); combined.set(new Uint8Array(ct), iv.length)
  return btoa(String.fromCharCode(...combined))
}

export async function decryptMessage(b64: string, sharedKey: CryptoKey): Promise<string> {
  try {
    const combined = Uint8Array.from(atob(b64), c => c.charCodeAt(0))
    const iv = combined.slice(0, 12); const ct = combined.slice(12)
    const pt = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, sharedKey, ct)
    return new TextDecoder().decode(pt)
  } catch { return '[decryption failed]' }
}

export async function queueOfflineMessage(msg: { conversation_id: string; client_id: string; kind: string; body: string }) {
  const db = await openOfflineDB()
  const tx = db.transaction('pending_messages', 'readwrite')
  tx.objectStore('pending_messages').add({ ...msg, queued_at: Date.now() })
  await new Promise<void>((r, j) => { tx.oncomplete = () => r(); tx.onerror = () => j(tx.error) })
  db.close()
}

export async function flushOfflineQueue(sendFn: (msg: any) => Promise<void>) {
  const db = await openOfflineDB()
  const tx = db.transaction('pending_messages', 'readonly')
  const req = tx.objectStore('pending_messages').getAll()
  const items = await new Promise<any[]>((r, j) => { req.onsuccess = () => r(req.result); req.onerror = () => j(req.error) })
  for (const item of items) {
    try { await sendFn(item); const dt = db.transaction('pending_messages', 'readwrite'); dt.objectStore('pending_messages').delete(item.id); await new Promise<void>((r, j) => { dt.oncomplete = () => r(); dt.onerror = () => j(dt.error) }) } catch { break }
  }
  db.close()
}

function openOfflineDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open('line_offline_queue', 1)
    req.onupgradeneeded = () => { const db = req.result; if (!db.objectStoreNames.contains('pending_messages')) db.createObjectStore('pending_messages', { keyPath: 'id', autoIncrement: true }) }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

export { openOfflineDB }
// --- Signal Protocol Key Management (IndexedDB) ---
const SIGNAL_DB_NAME = 'line_signal_protocol'
const SIGNAL_STORE = 'sessions'

async function openSignalDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(SIGNAL_DB_NAME, 1)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(SIGNAL_STORE)) {
        db.createObjectStore(SIGNAL_STORE, { keyPath: 'conversationId' })
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

async function storeSignalSession(conversationId: string, session: any): Promise<void> {
  const db = await openSignalDB()
  const tx = db.transaction(SIGNAL_STORE, 'readwrite')
  tx.objectStore(SIGNAL_STORE).put({ conversationId, ...session, updated_at: Date.now() })
  await new Promise<void>((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
  db.close()
}

async function loadSignalSession(conversationId: string): Promise<any | null> {
  const db = await openSignalDB()
  const tx = db.transaction(SIGNAL_STORE, 'readonly')
  const req = tx.objectStore(SIGNAL_STORE).get(conversationId)
  const result = await new Promise<any>((resolve, reject) => {
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
  db.close()
  return result || null
}

async function deleteSignalSession(conversationId: string): Promise<void> {
  const db = await openSignalDB()
  const tx = db.transaction(SIGNAL_STORE, 'readwrite')
  tx.objectStore(SIGNAL_STORE).delete(conversationId)
  await new Promise<void>((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
  db.close()
}

export { storeSignalSession, loadSignalSession, deleteSignalSession }