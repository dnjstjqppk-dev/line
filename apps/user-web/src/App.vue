<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { request, newClientID, wsURL, type Chat, type Message, type User, type LoginResult } from '../../shared/api'

const email = ref(''), password = ref(''), displayName = ref(''), registerMode = ref(false)
const token = ref(''), currentUser = ref<User | null>(null), error = ref(''), note = ref('')
const chats = ref<Chat[]>([]), selected = ref(''), messages = ref<Message[]>([])
const search = ref(''), found = ref<User[]>([]), friends = ref<User[]>([]), draft = ref(''), selectedName = ref('')
let socket: WebSocket | null = null
const api = <T,>(path: string, method = 'GET', body?: unknown) => request<T>(path, token.value || null, method, body)
const wrap = async (job: () => Promise<void>) => { error.value = ''; note.value = ''; try { await job() } catch (e) { error.value = (e as Error).message } }
async function authAction() { await wrap(async () => {
 if (registerMode.value) { await request('/api/v1/auth/register', null, 'POST', { email: email.value, password: password.value, display_name: displayName.value }); note.value = '계정 생성 완료 — 로그인하세요'; registerMode.value = false; return }
 const data = await request<LoginResult>('/api/v1/auth/login', null, 'POST', { email: email.value, password: password.value })
 token.value = data.access_token; currentUser.value = await api<User>('/api/v1/me'); password.value = ''
 await Promise.all([loadChats(), loadFriends()]); await connectRealtime()
}) }
async function connectRealtime() { if (socket) socket.close()
 const { ticket } = await api<{ ticket: string }>('/api/v1/ws-ticket', 'POST')
 socket = new WebSocket(wsURL(ticket))
 socket.onmessage = (event) => { try { const data = JSON.parse(event.data)
 if (data.type === 'message.created') { const msg = data.message as Message
  if (msg.conversation_id === selected.value && !messages.value.some((m) => m.id === msg.id)) messages.value.push(msg)
  void loadChats()
 }
 } catch { /* ignore invalid event */ } }
 socket.onclose = () => { if (token.value) note.value = '실시간 연결 종료: 재연결 버튼을 누르세요' }
}
async function loadChats() { const x = await api<{ conversations: Chat[] }>('/api/v1/conversations'); chats.value = x.conversations }
async function loadFriends() { const x = await api<{ friends: User[] }>('/api/v1/friends'); friends.value = x.friends }
async function lookup() { await wrap(async () => { const x = await api<{ users: User[] }>('/api/v1/users?search=' + encodeURIComponent(search.value)); found.value = x.users }) }
async function addFriend(id: string) { await wrap(async () => { await api('/api/v1/friends', 'POST', { friend_id: id }); await loadFriends(); note.value = '친구 추가 완료' }) }
async function openChat(id: string, title?: string) { selected.value = id; selectedName.value = title || id; await wrap(async () => {
 const x = await api<{ messages: Message[] }>(`/api/v1/conversations/${id}/messages`)
 messages.value = x.messages
 if (messages.value.length) await api(`/api/v1/conversations/${id}/read`, 'POST', { sequence: messages.value[messages.value.length - 1].sequence })
 await loadChats()
}) }
async function startDM(id: string, name: string) { await wrap(async () => {
 const c = await api<{ id: string }>('/api/v1/conversations', 'POST', { kind: 'direct', member_ids: [id], title: name })
 await loadChats(); await openChat(c.id, name)
}) }
async function send() { if (!selected.value || !draft.value.trim()) return; const body = draft.value; draft.value = ''
 await wrap(async () => { const data = await api<{ message: Message }>(`/api/v1/conversations/${selected.value}/messages`, 'POST', { client_id: newClientID(), kind: 'text', body })
 if (!messages.value.some(m => m.id === data.message.id)) messages.value.push(data.message)
 await loadChats()
 }); if (error.value) draft.value = body
}
function logout() { token.value = ''; currentUser.value = null; chats.value = []; selected.value = ''; messages.value = []; if (socket) socket.close(); socket = null }
onBeforeUnmount(() => { if (socket) socket.close() })
</script>
<template>
<div class="shell">
 <header><strong>Global Messenger</strong><span class="muted">독립 구현 메신저 · MVP</span><button v-if="token" @click="logout">로그아웃</button></header>
 <div v-if="error" role="alert" class="error">{{ error }}</div><p v-if="note" class="note">{{ note }}</p>
 <main v-if="!currentUser" class="auth card">
  <h2>{{ registerMode ? '회원가입' : '로그인' }}</h2>
  <form @submit.prevent="authAction">
   <label>이메일 <input v-model="email" required type="email" autocomplete="username" /></label>
   <label v-if="registerMode">이름 <input v-model="displayName" required /></label>
   <label>비밀번호 <input v-model="password" required type="password" :minlength="registerMode ? 12 : 1" autocomplete="current-password" /></label>
   <button class="primary" type="submit">{{ registerMode ? '계정 만들기' : '로그인' }}</button>
  </form><button @click="registerMode = !registerMode">{{ registerMode ? '로그인으로' : '새 계정 만들기' }}</button>
 </main>
 <main v-else class="layout">
  <aside class="sidebar">
   <div class="card profile"><strong>{{ currentUser.display_name }}</strong><small>{{ currentUser.email }}</small><button @click="wrap(connectRealtime)">실시간 재연결</button></div>
   <div class="card"><h3>사용자 검색</h3><form @submit.prevent="lookup" class="flex"><input v-model="search" placeholder="이름 2글자 이상" minlength="2" /><button>검색</button></form>
    <div v-for="u in found" :key="u.id" class="item"><span>{{ u.display_name }}</span><button @click="addFriend(u.id)">친구 추가</button></div>
   </div>
   <div class="card"><h3>친구</h3><div v-for="u in friends" :key="u.id" class="item"><span>{{ u.display_name }}</span><button @click="startDM(u.id,u.display_name)">대화</button></div></div>
   <div class="card"><h3>대화</h3><button v-for="c in chats" :key="c.id" class="chat-link" :class="{ active: selected === c.id }" @click="openChat(c.id,c.title)">{{ c.title || c.kind }} <small v-if="c.unread_count">{{ c.unread_count }} 새 메시지</small></button></div>
  </aside>
  <section class="conversation card" v-if="selected"><h2>{{ selectedName || '대화' }}</h2>
   <div class="messages" role="log" aria-live="polite"><div v-for="m in messages" :key="m.id" class="bubble" :class="{ mine: m.sender_id === currentUser.id }"><div>{{ m.body }}</div><small>{{ new Date(m.created_at).toLocaleTimeString() }}</small></div></div>
   <form class="composer" @submit.prevent="send"><input v-model="draft" aria-label="메시지" placeholder="메시지 입력" maxlength="20000" /><button class="primary">전송</button></form>
  </section>
  <section v-else class="card empty">친구를 선택하고 대화를 시작하세요.</section>
 </main>
 <footer>이 버전은 텍스트 채팅·인증·기본 파트너 기능만 구현합니다. 통화·E2EE·파일 전송·결제는 구현되지 않았습니다.</footer>
</div>
</template>
<style>
:root{font-family:Inter,system-ui,'Noto Sans KR',sans-serif;color:#172536;background:#f3f6fa}*{box-sizing:border-box}body{margin:0}button,input{font:inherit}button{border:1px solid #d5deeb;border-radius:9px;padding:9px 12px;background:#fff;cursor:pointer}button:hover{background:#eaf1fa}button.primary{background:#06754e;color:white;border-color:#06754e}input{border:1px solid #cad7e6;border-radius:9px;padding:10px;min-width:0;width:100%}label{display:grid;gap:6px;margin:12px 0}header{display:flex;align-items:center;gap:14px;padding:15px 24px;background:white;border-bottom:1px solid #e0e8ee}header button{margin-left:auto}.muted,small{color:#617489}.shell{min-height:100vh}h2{font-size:20px}h3{font-size:15px;margin:0 0 12px}.card{background:white;border:1px solid #e0e8ee;border-radius:12px;padding:16px}.auth{max-width:430px;margin:60px auto}.layout{display:grid;grid-template-columns:310px minmax(0,1fr);gap:15px;margin:15px;min-height:75vh}.sidebar{display:grid;align-content:start;gap:12px}.flex{display:flex;gap:6px}.item{display:flex;align-items:center;justify-content:space-between;gap:8px;margin:8px 0;padding:4px 0}.item button{font-size:12px}.chat-link{display:block;width:100%;text-align:left;margin:6px 0}.active{background:#d7f5e7}.conversation{display:flex;flex-direction:column;height:76vh}.messages{overflow:auto;flex:1;display:flex;flex-direction:column;gap:12px;padding:12px}.bubble{align-self:start;background:#e8eef6;max-width:78%;border-radius:14px;padding:12px;overflow-wrap:anywhere}.bubble.mine{align-self:end;background:#dcf8df}.bubble small{display:block;text-align:right;font-size:11px;margin-top:5px}.composer{display:flex;gap:8px;border-top:1px solid #eee;padding-top:13px}.empty{display:grid;place-items:center}.error,.note{max-width:800px;margin:10px auto;padding:10px;border-radius:8px}.error{background:#ffebeb;color:#923a36}.note{background:#e4f7ee}footer{text-align:center;color:#697b90;font-size:12px;padding:18px}@media(max-width:740px){.layout{grid-template-columns:1fr}.conversation{height:65vh}}
</style>
