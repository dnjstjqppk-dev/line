<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { request, newClientID, wsURL, type Chat, type Message, type User, type LoginResult } from '../../shared/api'

const email = ref(''), password = ref(''), displayName = ref(''), registerMode = ref(false)
const token = ref(''), currentUser = ref<User | null>(null), error = ref(''), note = ref('')
const chats = ref<Chat[]>([]), selected = ref(''), messages = ref<Message[]>([])
const search = ref(''), found = ref<User[]>([]), friends = ref<User[]>([]), draft = ref(''), selectedName = ref('')
const replyToID = ref(''), activeCall = ref<{ id: string; kind: string; state: string } | null>(null)
const qrChallengeID = ref(''), qrToken = ref(''), approveChallengeInput = ref(''), approveTokenInput = ref('')
const groupTitle = ref(''), selectedGroupMembers = ref<string[]>([])
let socket: WebSocket | null = null

const api = <T,>(path: string, method = 'GET', body?: unknown) => request<T>(path, token.value || null, method, body)
const wrap = async (job: () => Promise<void>) => {
  error.value = ''; note.value = ''
  try { await job() } catch (e) { error.value = (e as Error).message }
}

async function authAction() {
  await wrap(async () => {
    if (registerMode.value) {
      await request('/api/v1/auth/register', null, 'POST', { email: email.value, password: password.value, display_name: displayName.value })
      note.value = '계정 생성 완료 — 로그인하세요'
      registerMode.value = false
      return
    }
    const data = await request<LoginResult>('/api/v1/auth/login', null, 'POST', {
      email: email.value,
      password: password.value,
      device_name: 'Web/PC Messenger',
      platform: 'web',
    })
    token.value = data.access_token
    currentUser.value = await api<User>('/api/v1/me')
    password.value = ''
    await Promise.all([loadChats(), loadFriends()])
    await connectRealtime()
  })
}

async function createQRChallenge() {
  await wrap(async () => {
    const res = await request<{ challenge_id: string; qr_token: string }>('/api/v1/auth/qr/challenge', null, 'POST', {
      device_name: 'Windows/macOS PC Client',
      platform: 'desktop',
    })
    qrChallengeID.value = res.challenge_id
    qrToken.value = res.qr_token
    note.value = '모바일 앱에서 QR 코드(Challenge ID + Token)를 승인한 뒤 [QR 로그인 확인]을 누르세요.'
  })
}

async function pollQRLogin() {
  await wrap(async () => {
    const res = await request<LoginResult & { status: string }>('/api/v1/auth/qr/poll', null, 'POST', {
      challenge_id: qrChallengeID.value,
      qr_token: qrToken.value,
    })
    if (!res.access_token) {
      note.value = '아직 모바일 승인 대기 중입니다.'
      return
    }
    token.value = res.access_token
    currentUser.value = await api<User>('/api/v1/me')
    qrChallengeID.value = ''
    qrToken.value = ''
    await Promise.all([loadChats(), loadFriends()])
    await connectRealtime()
  })
}

async function approveDesktopQR() {
  await wrap(async () => {
    await api('/api/v1/auth/qr/approve', 'POST', {
      challenge_id: approveChallengeInput.value.trim(),
      qr_token: approveTokenInput.value.trim(),
    })
    approveChallengeInput.value = ''
    approveTokenInput.value = ''
    note.value = 'PC QR 로그인을 승인했습니다.'
  })
}

async function connectRealtime() {
  if (socket) socket.close()
  const { ticket } = await api<{ ticket: string }>('/api/v1/ws-ticket', 'POST')
  socket = new WebSocket(wsURL(ticket))
  socket.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      if (data.type === 'message.created') {
        const msg = data.message as Message
        if (msg.conversation_id === selected.value && !messages.value.some((m) => m.id === msg.id)) {
          messages.value.push(msg)
        }
        void loadChats()
      } else if (data.type === 'conversation.read') {
        void loadChats()
      } else if (data.type === 'call.ringing') {
        activeCall.value = { id: data.call_id, kind: data.kind, state: 'ringing' }
      } else if (data.type === 'call.state') {
        if (data.state === 'ended') activeCall.value = null
        else if (activeCall.value && activeCall.value.id === data.call_id) activeCall.value.state = data.state
      }
    } catch { /* ignore invalid event */ }
  }
  socket.onclose = () => { if (token.value) note.value = '실시간 연결 종료: 재연결 및 동기화 버튼을 누르세요' }
}

async function loadChats() { const x = await api<{ conversations: Chat[] }>('/api/v1/conversations'); chats.value = x.conversations }
async function loadFriends() { const x = await api<{ friends: User[] }>('/api/v1/friends'); friends.value = x.friends }
async function lookup() { await wrap(async () => { const x = await api<{ users: User[] }>('/api/v1/users?search=' + encodeURIComponent(search.value)); found.value = x.users }) }
async function addFriend(id: string) { await wrap(async () => { await api('/api/v1/friends', 'POST', { friend_id: id }); await loadFriends(); note.value = '친구 추가 완료' }) }

async function openChat(id: string, title?: string) {
  selected.value = id; selectedName.value = title || id
  await wrap(async () => {
    const x = await api<{ messages: Message[] }>(`/api/v1/conversations/${id}/messages`)
    messages.value = x.messages
    if (messages.value.length) {
      await api(`/api/v1/conversations/${id}/read`, 'POST', { sequence: messages.value[messages.value.length - 1].sequence })
    }
    await loadChats()
  })
}

async function syncOfflineMessages() {
  if (!selected.value) return
  await wrap(async () => {
    const lastSeq = messages.value.length ? messages.value[messages.value.length - 1].sequence : 0
    const x = await api<{ messages: Message[] }>(`/api/v1/conversations/${selected.value}/sync?after_sequence=${lastSeq}`)
    for (const m of x.messages) {
      if (!messages.value.some((ex) => ex.id === m.id)) messages.value.push(m)
    }
    await loadChats()
    note.value = `오프라인 누락 메시지 ${x.messages.length}건 동기화 완료`
  })
}

async function startDM(id: string, name: string) {
  await wrap(async () => {
    const c = await api<{ id: string }>('/api/v1/conversations', 'POST', { kind: 'direct', member_ids: [id], title: name })
    await loadChats(); await openChat(c.id, name)
  })
}

async function createGroupChat() {
  if (!groupTitle.value.trim() || selectedGroupMembers.value.length === 0) return
  await wrap(async () => {
    const c = await api<{ id: string }>('/api/v1/conversations', 'POST', {
      kind: 'group',
      title: groupTitle.value.trim(),
      member_ids: selectedGroupMembers.value,
    })
    const title = groupTitle.value.trim()
    groupTitle.value = ''; selectedGroupMembers.value = []
    await loadChats(); await openChat(c.id, title)
  })
}

async function send() {
  if (!selected.value || !draft.value.trim()) return
  const body = draft.value; const reply = replyToID.value
  draft.value = ''; replyToID.value = ''
  await wrap(async () => {
    const data = await api<{ message: Message }>(`/api/v1/conversations/${selected.value}/messages`, 'POST', {
      client_id: newClientID(),
      kind: 'text',
      body,
      ...(reply ? { reply_to: reply } : {}),
    })
    if (!messages.value.some((m) => m.id === data.message.id)) messages.value.push(data.message)
    await loadChats()
  })
  if (error.value) draft.value = body
}

async function sendSampleAttachment() {
  if (!selected.value) return
  await wrap(async () => {
    const att = await api<{ id: string; original_name: string }>('/api/v1/attachments', 'POST', {
      original_name: 'photo-attachment.png',
      mime_type: 'image/png',
      size_bytes: 12480,
      content_base64: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
    })
    const data = await api<{ message: Message }>(`/api/v1/conversations/${selected.value}/messages`, 'POST', {
      client_id: newClientID(),
      kind: 'image',
      body: att.original_name,
      metadata: { attachment_id: att.id, mime_type: 'image/png' },
    })
    if (!messages.value.some((m) => m.id === data.message.id)) messages.value.push(data.message)
    await loadChats()
  })
}

async function startCall(kind: 'voice' | 'video') {
  if (!selected.value) return
  await wrap(async () => {
    const c = await api<{ id: string; kind: string; state: string }>('/api/v1/calls', 'POST', {
      conversation_id: selected.value,
      kind,
      sdp_offer: 'v=0\r\no=- 100 2 IN IP4 127.0.0.1\r\ns=WebRTC-Call',
    })
    activeCall.value = c
  })
}

async function updateCall(state: 'connected' | 'ended') {
  if (!activeCall.value) return
  const callID = activeCall.value.id
  await wrap(async () => {
    await api(`/api/v1/calls/${callID}/state`, 'POST', { state, end_reason: state === 'ended' ? 'hangup' : '' })
    if (state === 'ended') activeCall.value = null
    else if (activeCall.value) activeCall.value.state = 'connected'
  })
}

function logout() {
  token.value = ''; currentUser.value = null; chats.value = []; selected.value = ''; messages.value = []
  if (socket) socket.close(); socket = null
}
onBeforeUnmount(() => { if (socket) socket.close() })
</script>
<template>
<div class="shell">
  <header>
    <strong>Global Messenger</strong>
    <span class="muted">실시간 멀티 디바이스 메신저 · Web & PC & Mobile</span>
    <nav class="console-nav" style="display:flex;gap:10px;margin-left:auto;align-items:center;font-size:13px">
      <a href="./admin/" target="_blank" style="text-decoration:none;color:#06754e;font-weight:600">관리자</a>
      <a href="./partner/" target="_blank" style="text-decoration:none;color:#06754e;font-weight:600">파트너</a>
      <a href="./developer/" target="_blank" style="text-decoration:none;color:#06754e;font-weight:600">개발자</a>
      <a href="./creator/" target="_blank" style="text-decoration:none;color:#06754e;font-weight:600">크리에이터</a>
      <a href="./ads/" target="_blank" style="text-decoration:none;color:#06754e;font-weight:600">광고주</a>
      <button v-if="token" @click="logout" style="margin-left:8px">로그아웃</button>
    </nav>
  </header>
  <div v-if="error" role="alert" class="error">{{ error }}</div>
  <p v-if="note" class="note">{{ note }}</p>

  <main v-if="!currentUser" class="auth card">
    <h2>{{ registerMode ? '회원가입' : '로그인' }}</h2>
    <div style="margin:10px 0;padding:8px 12px;background:#eef5fc;border-radius:8px;font-size:12px;color:#2c5282">
      <strong>💡 기본 데모 계정:</strong> <code>user@line.local</code> / <code>LineUser#2026!</code>
    </div>
    <form @submit.prevent="authAction">
      <label>이메일 <input v-model="email" required type="email" autocomplete="username" /></label>
      <label v-if="registerMode">이름 <input v-model="displayName" required /></label>
      <label>비밀번호 <input v-model="password" required type="password" :minlength="registerMode ? 12 : 1" autocomplete="current-password" /></label>
      <button class="primary" type="submit">{{ registerMode ? '계정 만들기' : '로그인' }}</button>
    </form>
    <div class="flex" style="margin-top:10px">
      <button @click="registerMode = !registerMode">{{ registerMode ? '로그인으로' : '새 계정 만들기' }}</button>
      <button @click="createQRChallenge">PC QR 코드 로그인 생성</button>
    </div>
    <div v-if="qrChallengeID" class="qr-box">
      <p><strong>QR Challenge ID:</strong> <code>{{ qrChallengeID }}</code></p>
      <p><strong>QR Token:</strong> <code>{{ qrToken }}</code></p>
      <button class="primary" @click="pollQRLogin">QR 로그인 확인</button>
    </div>
  </main>

  <main v-else class="layout">
    <aside class="sidebar">
      <div class="card profile">
        <strong>{{ currentUser.display_name }}</strong>
        <small>{{ currentUser.email }}</small>
        <div class="flex" style="margin-top:8px">
          <button @click="wrap(connectRealtime)">실시간 재연결</button>
          <button v-if="selected" @click="syncOfflineMessages">오프라인 동기화</button>
        </div>
      </div>

      <div class="card">
        <h3>PC QR 로그인 승인 (모바일 인증)</h3>
        <input v-model="approveChallengeInput" placeholder="QR Challenge ID" style="margin-bottom:6px" />
        <input v-model="approveTokenInput" placeholder="QR Token" style="margin-bottom:6px" />
        <button @click="approveDesktopQR">PC 로그인 승인</button>
      </div>

      <div class="card">
        <h3>사용자 검색</h3>
        <form @submit.prevent="lookup" class="flex">
          <input v-model="search" placeholder="이름 2글자 이상" minlength="2" />
          <button>검색</button>
        </form>
        <div v-for="u in found" :key="u.id" class="item">
          <span>{{ u.display_name }}</span>
          <button @click="addFriend(u.id)">친구 추가</button>
        </div>
      </div>

      <div class="card">
        <h3>친구 ({{ friends.length }})</h3>
        <div v-for="u in friends" :key="u.id" class="item">
          <label style="display:flex;align-items:center;gap:6px;margin:0">
            <input type="checkbox" :value="u.id" v-model="selectedGroupMembers" style="width:auto" />
            <span>{{ u.display_name }}</span>
          </label>
          <button @click="startDM(u.id, u.display_name)">1:1 대화</button>
        </div>
        <div v-if="selectedGroupMembers.length" style="margin-top:10px">
          <input v-model="groupTitle" placeholder="그룹 채팅방 이름" style="margin-bottom:6px" />
          <button class="primary" @click="createGroupChat">그룹 채팅 생성 ({{ selectedGroupMembers.length }}명)</button>
        </div>
      </div>

      <div class="card">
        <h3>대화 목록</h3>
        <button v-for="c in chats" :key="c.id" class="chat-link" :class="{ active: selected === c.id }" @click="openChat(c.id, c.title)">
          {{ c.title || c.kind }}
          <small v-if="c.unread_count">{{ c.unread_count }} 새 메시지</small>
        </button>
      </div>
    </aside>

    <section class="conversation card" v-if="selected">
      <div class="conv-header">
        <h2>{{ selectedName || '대화' }}</h2>
        <div class="flex">
          <button @click="startCall('voice')">음성통화</button>
          <button @click="startCall('video')">영상통화</button>
          <button @click="sendSampleAttachment">이미지/파일 전송</button>
        </div>
      </div>

      <div v-if="activeCall" class="call-banner">
        <span>통화 상태: <strong>{{ activeCall.kind.toUpperCase() }} ({{ activeCall.state }})</strong></span>
        <div class="flex">
          <button v-if="activeCall.state === 'ringing'" class="primary" @click="updateCall('connected')">수락</button>
          <button @click="updateCall('ended')">통화 종료</button>
        </div>
      </div>

      <div class="messages" role="log" aria-live="polite">
        <div v-for="m in messages" :key="m.id" class="bubble" :class="{ mine: m.sender_id === currentUser.id }">
          <small v-if="m.reply_to" class="muted">↩ 답장 메시지</small>
          <div>
            <strong v-if="m.kind !== 'text'">[{{ m.kind.toUpperCase() }}] </strong>{{ m.body }}
          </div>
          <div class="flex" style="justify-content:flex-end;margin-top:4px">
            <small>#{{ m.sequence }} · {{ new Date(m.created_at).toLocaleTimeString() }}</small>
            <button style="padding:2px 6px;font-size:11px" @click="replyToID = m.id">답장</button>
          </div>
        </div>
      </div>

      <div v-if="replyToID" class="note" style="margin:4px 0;display:flex;justify-content:space-between">
        <span>메시지 답장 모드</span>
        <button @click="replyToID = ''">취소</button>
      </div>

      <form class="composer" @submit.prevent="send">
        <input v-model="draft" aria-label="메시지" placeholder="메시지 입력" maxlength="20000" />
        <button class="primary">전송</button>
      </form>
    </section>
    <section v-else class="card empty">친구를 선택하고 1:1 또는 그룹 대화를 시작하세요.</section>
  </main>
</div>
</template>
<style>
:root{font-family:Inter,system-ui,'Noto Sans KR',sans-serif;color:#172536;background:#f3f6fa}*{box-sizing:border-box}body{margin:0}button,input{font:inherit}button{border:1px solid #d5deeb;border-radius:9px;padding:9px 12px;background:#fff;cursor:pointer}button:hover{background:#eaf1fa}button.primary{background:#06754e;color:white;border-color:#06754e}input{border:1px solid #cad7e6;border-radius:9px;padding:10px;min-width:0;width:100%}label{display:grid;gap:6px;margin:12px 0}header{display:flex;align-items:center;gap:14px;padding:15px 24px;background:white;border-bottom:1px solid #e0e8ee}header button{margin-left:auto}.muted,small{color:#617489}.shell{min-height:100vh}h2{font-size:20px;margin:0}h3{font-size:15px;margin:0 0 12px}.card{background:white;border:1px solid #e0e8ee;border-radius:12px;padding:16px}.auth{max-width:460px;margin:50px auto}.qr-box{margin-top:12px;padding:12px;background:#eef6f2;border-radius:8px;font-size:13px;overflow-wrap:anywhere}.layout{display:grid;grid-template-columns:340px minmax(0,1fr);gap:15px;margin:15px;min-height:80vh}.sidebar{display:grid;align-content:start;gap:12px}.flex{display:flex;gap:6px;align-items:center}.item{display:flex;align-items:center;justify-content:space-between;gap:8px;margin:8px 0;padding:4px 0}.item button{font-size:12px}.chat-link{display:block;width:100%;text-align:left;margin:6px 0}.active{background:#d7f5e7}.conversation{display:flex;flex-direction:column;height:80vh}.conv-header{display:flex;justify-content:space-between;align-items:center;padding-bottom:10px;border-bottom:1px solid #eee}.call-banner{display:flex;justify-content:space-between;align-items:center;background:#e6f4ea;padding:10px 14px;border-radius:8px;margin:8px 0}.messages{overflow:auto;flex:1;display:flex;flex-direction:column;gap:12px;padding:12px}.bubble{align-self:start;background:#e8eef6;max-width:78%;border-radius:14px;padding:12px;overflow-wrap:anywhere}.bubble.mine{align-self:end;background:#dcf8df}.composer{display:flex;gap:8px;border-top:1px solid #eee;padding-top:13px}.empty{display:grid;place-items:center}.error,.note{max-width:800px;margin:10px auto;padding:10px;border-radius:8px}.error{background:#ffebeb;color:#923a36}.note{background:#e4f7ee}@media(max-width:740px){.layout{grid-template-columns:1fr}.conversation{height:65vh}}
</style>
