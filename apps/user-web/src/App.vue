<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import {
  request,
  newClientID,
  wsURL,
  type Chat,
  type Message,
  type User,
  type LoginResult
} from '../../shared/api'

// --- Auth & Session State ---
const email = ref('')
const password = ref('')
const displayName = ref('')
const registerMode = ref(false)
const token = ref('')
const currentUser = ref<User | null>(null)
const error = ref('')
const note = ref('')

// --- Navigation Tabs ---
// 'home' (Friends/Profile), 'chats', 'voom' (Timeline/Stories), 'openchat', 'wallet' (Pay/Services)
const activeNav = ref<'home' | 'chats' | 'voom' | 'openchat' | 'wallet'>('chats')
const activeTheme = ref<'mint' | 'ocean' | 'sunset' | 'sakura' | 'midnight'>('mint')
const darkMode = ref(false)

// --- Messenger Domain Data ---
const chats = ref<Chat[]>([])
const selectedChatId = ref('')
const selectedChat = computed(() => chats.value.find(c => c.id === selectedChatId.value) || null)
const messages = ref<Message[]>([])
const friends = ref<User[]>([])
const officialAccounts = ref<Array<{ id: string; name: string; handle: string; description: string; color: string; followed: boolean }>>([])
const searchKeyword = ref('')
const foundUsers = ref<User[]>([])
const draftMessage = ref('')
const replyToMessage = ref<Message | null>(null)
const activeDrawer = ref<'none' | 'notes' | 'albums' | 'events' | 'media'>('none')

// --- Pinned Announcements & Polls ---
const pinnedList = ref<{ message_id: string; pinned_by: string; pinned_at: string; body?: string }[]>([])
const pinnedExpanded = ref(false)
const activePolls = ref<Array<{ id: string; question: string; options: { id: string; text: string; votes: number; voted?: boolean }[]; closed?: boolean }>>([])
const chatNotes = ref<Array<{ id: string; title: string; body: string; author: User; created_at: string }>>([])
const chatEvents = ref<Array<{ id: string; title: string; starts_at: string; location: string; my_rsvp?: string; creator: User }>>([])

// --- Keep Vault ---
const keepItems = ref<Array<{ id: string; body: string; kind: string; created_at: string; message_id?: string }>>([])
const keepFilter = ref<'all' | 'text' | 'image' | 'link'>('all')
const keepSearch = ref('')
const showKeepModal = ref(false)

// --- VOOM / Timeline & Stories ---
const stories = ref<Array<{ id: string; author: User; body: string; bg: string; created_at: string; viewed?: boolean }>>([])
const voomVideos = ref<Array<{ id: string; title: string; emoji: string; bg: string; author: User; like_count: number; liked?: boolean; comments: Array<{ id: string; author: User; body: string }> }>>([])
const newStoryText = ref('')
const showNewStoryModal = ref(false)

// --- Wallet & Pay ---
const walletBalance = ref(50000)
const walletTx = ref<Array<{ id: string; type: string; amount: number; note?: string; counterparty?: string; created_at: string }>>([])
const showSendMoneyModal = ref(false)
const showChargeModal = ref(false)
const sendTargetFriend = ref<User | null>(null)
const sendAmountInput = ref(10000)
const sendMemoInput = ref('')

// --- Store / Stickers & Themes ---
const stickerPacks = ref<Array<{ id: string; title: string; price_minor: number; stickers: string[]; owned?: boolean }>>([])
const showStickerPicker = ref(false)
const activeStickerTab = ref(0)
const stickersList = [
  { id: 'pack-brown', name: '브라운 & 프렌즈', items: ['🐻', '🐰', '🐥', '🐸', '🐱', '🐶', '🐼', '🐨', '🦁', '🐯', '🦊', '🦄'] },
  { id: 'pack-emotions', name: '표정 & 감정', items: ['😀', '😂', '😍', '🥳', '😎', '😭', '😡', '🤔', '😴', '😱', '🤩', '😇'] },
  { id: 'pack-daily', name: '일상 & 음식', items: ['☕', '🍰', '🍕', '🍔', '🍣', '🍜', '🍺', '🍙', '🎉', '🎁', '💖', '👍'] },
  { id: 'pack-work', name: '직장인 & 개발', items: ['💻', '⌨️', '📱', '🚀', '🔥', '✨', '💯', '☕', '💪', '🙏', '🆗', '✅'] }
]

// --- Calling Simulation ---
const activeCall = ref<{ id: string; kind: 'voice' | 'video'; state: 'ringing' | 'connected' | 'ended'; title: string } | null>(null)
const callDuration = ref(0)
const isCallMuted = ref(false)
const isCallVideoOff = ref(false)
const isCallSpeakerOn = ref(true)
let callTimer: ReturnType<typeof setInterval> | null = null

// --- Modals & Popups ---
const activeProfileUser = ref<User | null>(null)
const showCreateGroupModal = ref(false)
const groupTitleInput = ref('')
const selectedFriendsForGroup = ref<string[]>([])
const showAddFriendModal = ref(false)
const addFriendHandleInput = ref('')
const showSettingsModal = ref(false)
const showPollCreateModal = ref(false)
const newPollQuestion = ref('')
const newPollOption1 = ref('')
const newPollOption2 = ref('')
const newPollOption3 = ref('')
const showEventCreateModal = ref(false)
const newEventTitle = ref('')
const newEventDate = ref('')
const newEventLocation = ref('')

// --- QR Login ---
const qrChallengeID = ref('')
const qrToken = ref('')
const approveChallengeInput = ref('')
const approveTokenInput = ref('')

// --- Realtime WebSocket ---
let socket: WebSocket | null = null

// --- Helper Functions ---
const api = <T,>(path: string, method = 'GET', body?: unknown) => request<T>(path, token.value || null, method, body)
const wrap = async (job: () => Promise<void>) => {
  error.value = ''
  note.value = ''
  try {
    await job()
  } catch (e) {
    error.value = (e as Error).message
  }
}

// Format duration mm:ss
function formatDuration(sec: number) {
  const m = Math.floor(sec / 60).toString().padStart(2, '0')
  const s = (sec % 60).toString().padStart(2, '0')
  return `${m}:${s}`
}

// Format time
function formatTime(iso: string) {
  if (!iso) return ''
  const d = new Date(iso)
  return d.toLocaleTimeString('ko-KR', { hour: 'numeric', minute: '2-digit', hour12: true })
}

// Compute total unread count
const totalUnreadCount = computed(() => chats.value.reduce((sum, c) => sum + (c.unread_count || 0), 0))

// Chat search / filter
const chatFilter = ref<'all' | 'direct' | 'group' | 'unread'>('all')
const filteredChats = computed(() => {
  return chats.value.filter(c => {
    if (chatFilter.value === 'direct' && c.kind !== 'direct') return false
    if (chatFilter.value === 'group' && c.kind !== 'group') return false
    if (chatFilter.value === 'unread' && (!c.unread_count || c.unread_count === 0)) return false
    if (searchKeyword.value.trim()) {
      const q = searchKeyword.value.toLowerCase()
      return (c.title || '').toLowerCase().includes(q) || (c.last_message?.body || '').toLowerCase().includes(q)
    }
    return true
  })
})

// Keep filtered items
const filteredKeepItems = computed(() => {
  return keepItems.value.filter(item => {
    const text = (item.body || '').toLowerCase()
    if (keepSearch.value && !text.includes(keepSearch.value.toLowerCase())) return false
    if (keepFilter.value === 'text') return item.kind === 'text' && !text.startsWith('http')
    if (keepFilter.value === 'image') return item.kind === 'image' || item.kind === 'photo'
    if (keepFilter.value === 'link') return text.startsWith('http') || text.includes('www.')
    return true
  })
})

// --- Auth Actions ---
async function authAction() {
  await wrap(async () => {
    if (registerMode.value) {
      await request('/api/v1/auth/register', null, 'POST', {
        email: email.value.trim(),
        password: password.value,
        display_name: displayName.value.trim() || email.value.split('@')[0]
      })
      note.value = '라인 회원가입이 완료되었습니다. 로그인하세요!'
      registerMode.value = false
      return
    }
    const data = await request<LoginResult>('/api/v1/auth/login', null, 'POST', {
      email: email.value.trim(),
      password: password.value,
      device_name: 'LINE Web Desktop Client',
      platform: 'web'
    })
    token.value = data.access_token
    currentUser.value = await api<User>('/api/v1/me')
    password.value = ''
    await loadInitialData()
    await connectRealtime()
  })
}

async function quickDemoLogin(demoEmail: string, demoPass: string) {
  email.value = demoEmail
  password.value = demoPass
  registerMode.value = false
  await authAction()
}

async function loadInitialData() {
  await Promise.all([
    loadChats(),
    loadFriends(),
    loadOfficialAccounts(),
    loadKeep(),
    loadWallet(),
    loadTimelineStories(),
    loadStoreCatalog()
  ])
}

// --- Realtime WebSocket Connection ---
async function connectRealtime() {
  if (socket) {
    socket.close()
    socket = null
  }
  try {
    const { ticket } = await api<{ ticket: string }>('/api/v1/ws-ticket', 'POST')
    socket = new WebSocket(wsURL(ticket))
    socket.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data.type === 'message.created') {
          const msg = data.message as Message
          if (msg.conversation_id === selectedChatId.value && !messages.value.some(m => m.id === msg.id)) {
            messages.value.push(msg)
          }
          void loadChats()
        } else if (data.type === 'message.updated') {
          const target = messages.value.find(m => m.id === data.message_id)
          if (target) target.body = data.body
        } else if (data.type === 'message.unsent') {
          const target = messages.value.find(m => m.id === data.message_id)
          if (target) {
            target.unsent = true
            target.body = '메시지를 취소했습니다'
          }
        } else if (data.type === 'message.reaction') {
          const target = messages.value.find(m => m.id === data.message_id)
          if (target) target.reactions = data.reactions
        } else if (data.type === 'conversation.read') {
          void loadChats()
        } else if (data.type === 'call.ringing') {
          activeCall.value = { id: data.call_id, kind: data.kind || 'voice', state: 'ringing', title: '수신된 통화' }
        } else if (data.type === 'call.state') {
          if (data.state === 'ended') {
            endCallLocally()
          } else if (activeCall.value && activeCall.value.id === data.call_id) {
            activeCall.value.state = 'connected'
            startCallTimer()
          }
        } else if (data.type === 'poll.updated') {
          if (data.conversation_id === selectedChatId.value) void loadPolls(selectedChatId.value)
        }
      } catch {
        // ignore malformed ws message
      }
    }
  } catch {
    // ticket error fallback
  }
}

// --- Data Fetchers ---
async function loadChats() {
  try {
    const res = await api<{ conversations: Chat[] }>('/api/v1/conversations')
    chats.value = res.conversations || []
  } catch {}
}

async function loadFriends() {
  try {
    const res = await api<{ friends: User[] }>('/api/v1/friends')
    friends.value = res.friends || []
  } catch {}
}

async function loadOfficialAccounts() {
  try {
    const res = await api<{ accounts: any[] }>('/api/v1/official-accounts')
    officialAccounts.value = res.accounts || []
  } catch {}
}

async function loadKeep() {
  try {
    const res = await api<{ saved_messages: any[] }>('/api/v1/saved-messages')
    keepItems.value = res.saved_messages || []
  } catch {}
}

async function loadWallet() {
  try {
    const res = await api<{ balance: number; transactions: any[] }>('/api/v1/wallet')
    walletBalance.value = res.balance || 0
    walletTx.value = res.transactions || []
  } catch {}
}

async function loadTimelineStories() {
  try {
    const [sRes, vRes] = await Promise.all([
      api<{ stories: any[] }>('/api/v1/stories'),
      api<{ videos: any[] }>('/api/v1/voom')
    ])
    stories.value = sRes.stories || []
    voomVideos.value = vRes.videos || []
  } catch {}
}

async function loadStoreCatalog() {
  try {
    const res = await api<{ packs: any[] }>('/api/v1/store/stickers')
    stickerPacks.value = res.packs || []
  } catch {}
}

// --- Chat Actions ---
let chatSelectVersion = 0
async function selectChat(convId: string) {
  selectedChatId.value = convId
  replyToMessage.value = null
  activeDrawer.value = 'none'
  const version = ++chatSelectVersion
  await wrap(async () => {
    const [msgRes, pinsRes] = await Promise.all([
      api<{ messages: Message[] }>(`/api/v1/conversations/${convId}/messages?limit=50`),
      api<{ pins: any[] }>(`/api/v1/conversations/${convId}/pins`)
    ])
    if (selectedChatId.value !== convId || version !== chatSelectVersion) return
    messages.value = msgRes.messages || []
    pinnedList.value = pinsRes.pins || []
    if (messages.value.length > 0) {
      const last = messages.value[messages.value.length - 1]
      await api(`/api/v1/conversations/${convId}/read`, 'POST', { sequence: last.sequence })
    }
    await Promise.all([loadChats(), loadPolls(convId)])
  })
}

let pollLoadVersion = 0
async function loadPolls(convId: string) {
  const version = ++pollLoadVersion
  try {
    const res = await api<{ polls: any[] }>(`/api/v1/conversations/${convId}/polls`)
    if (selectedChatId.value === convId && version === pollLoadVersion) {
      activePolls.value = res.polls || []
    }
  } catch {}
}

let noteLoadVersion = 0
async function loadNotes(convId: string) {
  const version = ++noteLoadVersion
  try {
    const res = await api<{ notes: any[] }>(`/api/v1/conversations/${convId}/notes`)
    if (selectedChatId.value === convId && version === noteLoadVersion) {
      chatNotes.value = res.notes || []
    }
  } catch {}
}

let eventLoadVersion = 0
async function loadEvents(convId: string) {
  const version = ++eventLoadVersion
  try {
    const res = await api<{ events: any[] }>(`/api/v1/conversations/${convId}/events`)
    if (selectedChatId.value === convId && version === eventLoadVersion) {
      chatEvents.value = res.events || []
    }
  } catch {}
}

function toggleDrawer(tab: 'notes' | 'albums' | 'events' | 'media') {
  if (activeDrawer.value === tab) {
    activeDrawer.value = 'none'
  } else {
    activeDrawer.value = tab
    if (tab === 'notes' && selectedChatId.value) loadNotes(selectedChatId.value)
    if (tab === 'events' && selectedChatId.value) loadEvents(selectedChatId.value)
  }
}

// --- Messaging ---
async function sendMessage() {
  if (!selectedChatId.value || !draftMessage.value.trim()) return
  const body = draftMessage.value.trim()
  const replyId = replyToMessage.value?.id
  draftMessage.value = ''
  replyToMessage.value = null
  await wrap(async () => {
    const res = await api<{ message: Message }>(`/api/v1/conversations/${selectedChatId.value}/messages`, 'POST', {
      client_id: newClientID(),
      kind: 'text',
      body,
      ...(replyId ? { reply_to: replyId } : {})
    })
    if (!messages.value.some(m => m.id === res.message.id)) {
      messages.value.push(res.message)
    }
    await loadChats()
  })
}

async function sendSticker(stickerChar: string) {
  if (!selectedChatId.value) return
  showStickerPicker.value = false
  await wrap(async () => {
    const res = await api<{ message: Message }>(`/api/v1/conversations/${selectedChatId.value}/messages`, 'POST', {
      client_id: newClientID(),
      kind: 'sticker',
      body: stickerChar
    })
    if (!messages.value.some(m => m.id === res.message.id)) {
      messages.value.push(res.message)
    }
    await loadChats()
  })
}

async function sendAttachmentMock(kind: 'image' | 'voice' | 'file') {
  if (!selectedChatId.value) return
  await wrap(async () => {
    const names = { image: 'photo_summer.jpg', voice: 'voice_note.m4a', file: 'project_brief.pdf' }
    const mimes = { image: 'image/jpeg', voice: 'audio/m4a', file: 'application/pdf' }
    const att = await api<{ id: string; original_name: string }>('/api/v1/attachments', 'POST', {
      conversation_id: selectedChatId.value,
      original_name: names[kind],
      mime_type: mimes[kind],
      content_base64: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='
    })
    const res = await api<{ message: Message }>(`/api/v1/conversations/${selectedChatId.value}/messages`, 'POST', {
      client_id: newClientID(),
      kind,
      body: att.original_name,
      metadata: { attachment_id: att.id, mime_type: mimes[kind], duration: kind === 'voice' ? 7 : undefined }
    })
    if (!messages.value.some(m => m.id === res.message.id)) {
      messages.value.push(res.message)
    }
    await loadChats()
  })
}

// React to message with LINE's 6 emoji reactions
async function reactToMessage(msgId: string, emoji: string) {
  if (!selectedChatId.value) return
  await wrap(async () => {
    const res = await api<{ reactions: Record<string, number> }>(`/api/v1/conversations/${selectedChatId.value}/messages/${msgId}/reactions`, 'POST', { reaction: emoji })
    const target = messages.value.find(m => m.id === msgId)
    if (target) target.reactions = res.reactions
  })
}

// Unsend message within 24h
async function unsendMessage(msgId: string) {
  if (!selectedChatId.value) return
  await wrap(async () => {
    await api(`/api/v1/conversations/${selectedChatId.value}/messages/${msgId}`, 'DELETE')
    const target = messages.value.find(m => m.id === msgId)
    if (target) {
      target.unsent = true
      target.body = '메시지를 취소했습니다'
    }
    note.value = '메시지를 성공적으로 전송 취소했습니다.'
  })
}

// Pin message as Announcement
async function pinMessage(msgId: string) {
  if (!selectedChatId.value) return
  await wrap(async () => {
    await api(`/api/v1/conversations/${selectedChatId.value}/pins`, 'POST', { message_id: msgId })
    const res = await api<{ pins: any[] }>(`/api/v1/conversations/${selectedChatId.value}/pins`)
    pinnedList.value = res.pins || []
    note.value = '대화방 상단 공지로 등록했습니다.'
  })
}

async function unpinMessage(msgId: string) {
  if (!selectedChatId.value) return
  await wrap(async () => {
    await api(`/api/v1/conversations/${selectedChatId.value}/pins/${msgId}`, 'DELETE')
    pinnedList.value = pinnedList.value.filter(p => p.message_id !== msgId)
    note.value = '공지를 내렸습니다.'
  })
}

// Save message to Keep
async function saveToKeep(msg: Message) {
  await wrap(async () => {
    const item = await api<any>('/api/v1/saved-messages', 'POST', {
      body: msg.body,
      kind: msg.kind,
      message_id: msg.id
    })
    keepItems.value.unshift(item)
    note.value = '라인 Keep 보관함에 저장되었습니다.'
  })
}

async function deleteFromKeep(item: any) {
  const sid = item.id || item.message_id
  if (!sid) return
  await wrap(async () => {
    await api(`/api/v1/saved-messages/${sid}`, 'DELETE')
    keepItems.value = keepItems.value.filter(k => (k.id || k.message_id) !== sid)
    note.value = 'Keep 보관함에서 항목을 삭제했습니다.'
  })
}

// Copy text helper
function copyMessageText(text: string) {
  navigator.clipboard.writeText(text)
  note.value = '메시지가 클립보드에 복사되었습니다.'
}

// Reply message helper
function startReply(msg: Message) {
  replyToMessage.value = msg
}

// --- Polls & Events ---
async function createPoll() {
  if (!selectedChatId.value || !newPollQuestion.value.trim() || !newPollOption1.value.trim() || !newPollOption2.value.trim()) return
  const convId = selectedChatId.value
  await wrap(async () => {
    const opts = [newPollOption1.value.trim(), newPollOption2.value.trim()]
    if (newPollOption3.value.trim()) opts.push(newPollOption3.value.trim())
    await api<any>(`/api/v1/conversations/${convId}/polls`, 'POST', {
      question: newPollQuestion.value.trim(),
      options: opts
    })
    if (selectedChatId.value === convId) await loadPolls(convId)
    newPollQuestion.value = ''
    newPollOption1.value = ''
    newPollOption2.value = ''
    newPollOption3.value = ''
    showPollCreateModal.value = false
    note.value = '새 투표가 생성되었습니다.'
  })
}

async function votePoll(pollId: string, optId: string) {
  await wrap(async () => {
    const res = await api<{ poll: any }>(`/api/v1/polls/${pollId}/vote`, 'POST', { option_id: optId })
    const idx = activePolls.value.findIndex(p => p.id === pollId)
    if (idx !== -1 && res.poll) {
      activePolls.value[idx] = res.poll
    }
  })
}

async function createEvent() {
  if (!selectedChatId.value || !newEventTitle.value.trim() || !newEventDate.value) return
  await wrap(async () => {
    const ev = await api<any>(`/api/v1/conversations/${selectedChatId.value}/events`, 'POST', {
      title: newEventTitle.value.trim(),
      starts_at: newEventDate.value,
      location: newEventLocation.value.trim()
    })
    chatEvents.value.unshift(ev)
    newEventTitle.value = ''
    newEventDate.value = ''
    newEventLocation.value = ''
    showEventCreateModal.value = false
    note.value = '대화방 일정이 등록되었습니다.'
  })
}

// --- Calling (Voice & Video) ---
async function startCall(kind: 'voice' | 'video') {
  if (!selectedChatId.value) return
  const convTitle = selectedChat.value?.title || '통화'
  await wrap(async () => {
    const c = await api<{ id: string; kind: 'voice' | 'video'; state: 'ringing' }>('/api/v1/calls', 'POST', {
      conversation_id: selectedChatId.value,
      kind
    })
    activeCall.value = { id: c.id, kind, state: 'ringing', title: convTitle }
    callDuration.value = 0
  })
}

async function updateCallState(state: 'connected' | 'ended') {
  if (!activeCall.value) return
  const cid = activeCall.value.id
  await wrap(async () => {
    await api(`/api/v1/calls/${cid}/state`, 'POST', { state })
    if (state === 'ended') {
      endCallLocally()
    } else if (activeCall.value) {
      activeCall.value.state = 'connected'
      startCallTimer()
    }
  })
}

function startCallTimer() {
  if (callTimer) clearInterval(callTimer)
  callDuration.value = 0
  callTimer = setInterval(() => { callDuration.value++ }, 1000)
}

function endCallLocally() {
  if (callTimer) {
    clearInterval(callTimer)
    callTimer = null
  }
  activeCall.value = null
  callDuration.value = 0
}

// --- Wallet & Pay ---
function openSendMoneyModal(friend: User) {
  sendTargetFriend.value = friend
  sendAmountInput.value = 10000
  sendMemoInput.value = '커피값 보냅니다 ☕'
  showSendMoneyModal.value = true
}

async function executeSendMoney() {
  if (!sendTargetFriend.value || sendAmountInput.value <= 0) return
  await wrap(async () => {
    const res = await api<{ balance: number }>('/api/v1/wallet/send', 'POST', {
      to_user_id: sendTargetFriend.value!.id,
      amount: sendAmountInput.value,
      note: sendMemoInput.value
    })
    walletBalance.value = res.balance
    showSendMoneyModal.value = false
    note.value = `${sendTargetFriend.value!.display_name}님께 ${sendAmountInput.value.toLocaleString()}원을 송금했습니다.`
    await Promise.all([loadWallet(), loadChats()])
  })
}

async function executeChargeMoney(amount: number) {
  await wrap(async () => {
    const res = await api<{ balance: number }>('/api/v1/wallet/charge', 'POST', { amount })
    walletBalance.value = res.balance
    showChargeModal.value = false
    note.value = `LINE Pay 잔액이 ${amount.toLocaleString()}원 충전되었습니다.`
    await loadWallet()
  })
}

// --- Friends & Direct Chat ---
async function startDirectChat(friend: User) {
  await wrap(async () => {
    const c = await api<{ id: string }>('/api/v1/conversations', 'POST', {
      kind: 'direct',
      member_ids: [friend.id],
      title: friend.display_name
    })
    await loadChats()
    activeNav.value = 'chats'
    await selectChat(c.id)
  })
}

async function createGroupChat() {
  if (!groupTitleInput.value.trim() || selectedFriendsForGroup.value.length === 0) return
  await wrap(async () => {
    const c = await api<{ id: string }>('/api/v1/conversations', 'POST', {
      kind: 'group',
      title: groupTitleInput.value.trim(),
      member_ids: selectedFriendsForGroup.value
    })
    groupTitleInput.value = ''
    selectedFriendsForGroup.value = []
    showCreateGroupModal.value = false
    await loadChats()
    activeNav.value = 'chats'
    await selectChat(c.id)
    note.value = '새 그룹 채팅방이 생성되었습니다.'
  })
}

async function addFriendByHandle() {
  if (!addFriendHandleInput.value.trim()) return
  await wrap(async () => {
    await api('/api/v1/friends', 'POST', { handle: addFriendHandleInput.value.trim() })
    addFriendHandleInput.value = ''
    showAddFriendModal.value = false
    await loadFriends()
    note.value = '친구가 추가되었습니다.'
  })
}

// --- Stories & VOOM ---
async function publishStory() {
  if (!newStoryText.value.trim()) return
  await wrap(async () => {
    const bgs = [
      'linear-gradient(135deg, #06C755, #0ea5e9)',
      'linear-gradient(135deg, #f97316, #ec4899)',
      'linear-gradient(135deg, #6366f1, #8b5cf6)'
    ]
    const chosenBg = bgs[Math.floor(Math.random() * bgs.length)]
    await api('/api/v1/stories', 'POST', {
      body: newStoryText.value.trim(),
      bg: chosenBg
    })
    newStoryText.value = ''
    showNewStoryModal.value = false
    note.value = '내 24시간 스토리가 등록되었습니다.'
    await loadTimelineStories()
  })
}

async function likeVoomPost(id: string) {
  await wrap(async () => {
    const res = await api<any>(`/api/v1/voom/${id}/like`, 'POST')
    const item = voomVideos.value.find(v => v.id === id)
    if (item && res) {
      item.like_count = res.like_count
      item.liked = res.liked
    }
  })
}

// --- Settings & Themes ---
function switchTheme(theme: 'mint' | 'ocean' | 'sunset' | 'sakura' | 'midnight') {
  activeTheme.value = theme
  darkMode.value = theme === 'midnight'
  note.value = `테마가 [${theme.toUpperCase()}]로 변경되었습니다.`
}

// --- Logout ---
function logout() {
  token.value = ''
  currentUser.value = null
  chats.value = []
  selectedChatId.value = ''
  messages.value = []
  if (callTimer) clearInterval(callTimer)
  if (socket) socket.close()
  socket = null
}

onBeforeUnmount(() => {
  if (callTimer) clearInterval(callTimer)
  if (socket) socket.close()
})
</script>

<template>
  <div class="line-app" :class="[`theme-${activeTheme}`, { 'dark-mode': darkMode }]">
    <!-- Top Global Service Navigation Bar -->
    <header class="line-header">
      <div class="brand">
        <span class="line-badge-icon">LINE</span>
        <span class="brand-title">LINE Global Messenger</span>
        <span class="version-badge">v2026.1 Enterprise</span>
      </div>
      <div class="header-consoles">
        <a href="./admin/" target="_blank" class="console-link">👑 관리자</a>
        <a href="./partner/" target="_blank" class="console-link">🏢 파트너</a>
        <a href="./developer/" target="_blank" class="console-link">💻 개발자</a>
        <a href="./creator/" target="_blank" class="console-link">🎨 크리에이터</a>
        <a href="./ads/" target="_blank" class="console-link">📢 광고주</a>
        <button v-if="token" class="btn-logout" @click="logout">로그아웃</button>
      </div>
    </header>

    <!-- Global Notice / Alert Banner -->
    <div v-if="error" class="alert-banner error" role="alert">
      <span>⚠️ {{ error }}</span>
      <button class="btn-dismiss" @click="error = ''">✕</button>
    </div>
    <div v-if="note" class="alert-banner note" role="status">
      <span>💡 {{ note }}</span>
      <button class="btn-dismiss" @click="note = ''">✕</button>
    </div>

    <!-- 1. LOGIN / REGISTER VIEW (LINE Authenticator UI) -->
    <main v-if="!currentUser" class="auth-container">
      <div class="auth-card">
        <div class="auth-logo-area">
          <div class="line-big-logo">LINE</div>
          <h1>라인 메신저</h1>
          <p class="auth-subtitle">친구, 가족, 세상과 연결되는 자유로운 글로벌 메신저</p>
        </div>

        <div class="auth-demo-box">
          <div class="demo-title">💡 클릭하여 원클릭 데모 로그인</div>
          <div class="demo-buttons">
            <button class="btn-demo" @click="quickDemoLogin('user@line.local', 'LineUser#2026!')">
              👤 일반 사용자 (Demo User)
            </button>
            <button class="btn-demo" @click="quickDemoLogin('alice@line-test.local', 'AliceStrongPass#2026')">
              👩 앨리스 (Alice Kim)
            </button>
            <button class="btn-demo" @click="quickDemoLogin('admin@line.local', 'LineAdmin#2026!')">
              👑 시스템 관리자 (Admin)
            </button>
          </div>
        </div>

        <form class="auth-form" @submit.prevent="authAction">
          <div class="form-group">
            <label>이메일 계정</label>
            <input v-model="email" type="email" placeholder="example@line.local" required autocomplete="username" />
          </div>

          <div v-if="registerMode" class="form-group">
            <label>이름 (프로필 닉네임)</label>
            <input v-model="displayName" type="text" placeholder="홍길동" required />
          </div>

          <div class="form-group">
            <label>비밀번호</label>
            <input v-model="password" type="password" placeholder="••••••••••••" required autocomplete="current-password" />
          </div>

          <button class="btn-primary line-btn-submit" type="submit">
            {{ registerMode ? '라인 계정 만들기' : '로그인' }}
          </button>
        </form>

        <div class="auth-footer-actions">
          <button class="btn-link" @click="registerMode = !registerMode">
            {{ registerMode ? '이미 계정이 있으신가요? 로그인' : '새 라인 계정 가입하기' }}
          </button>
        </div>
      </div>
    </main>

    <!-- 2. MAIN LOGGED-IN MESSENGER UI (LINE 3-PANE LAYOUT) -->
    <main v-else class="messenger-shell">
      <!-- (Column 1) Leftmost LINE Icon Dock -->
      <nav class="line-dock">
        <div class="dock-top">
          <div class="dock-app-icon" title="LINE Global">L</div>

          <button class="dock-btn" :class="{ active: activeNav === 'home' }" title="홈 / 친구" @click="activeNav = 'home'">
            <span class="icon">👤</span>
            <span class="label">홈</span>
          </button>

          <button class="dock-btn" :class="{ active: activeNav === 'chats' }" title="대화" @click="activeNav = 'chats'">
            <span class="icon">💬</span>
            <span class="label">대화</span>
            <span v-if="totalUnreadCount > 0" class="dock-badge">{{ totalUnreadCount }}</span>
          </button>

          <button class="dock-btn" :class="{ active: activeNav === 'voom' }" title="VOOM (타임라인/스토리)" @click="activeNav = 'voom'">
            <span class="icon">🎬</span>
            <span class="label">VOOM</span>
          </button>

          <button class="dock-btn" :class="{ active: activeNav === 'openchat' }" title="오픈채팅" @click="activeNav = 'openchat'">
            <span class="icon">🌐</span>
            <span class="label">오픈채팅</span>
          </button>

          <button class="dock-btn" :class="{ active: activeNav === 'wallet' }" title="더보기 & Pay" @click="activeNav = 'wallet'">
            <span class="icon">👛</span>
            <span class="label">Pay/더보기</span>
          </button>
        </div>

        <div class="dock-bottom">
          <button class="dock-btn" title="Keep 보관함" @click="showKeepModal = true">
            <span class="icon">💾</span>
            <span class="label">Keep</span>
          </button>
          <button class="dock-btn" title="설정" @click="showSettingsModal = true">
            <span class="icon">⚙️</span>
            <span class="label">설정</span>
          </button>
        </div>
      </nav>

      <!-- (Column 2) Second Sidebar List Pane -->
      <aside class="line-sidebar">
        <!-- TAB 1: HOME (Profile, Favorites, Birthday, Friends, OA) -->
        <div v-if="activeNav === 'home'" class="sidebar-content">
          <div class="sidebar-header">
            <h2>친구</h2>
            <button class="btn-icon" title="친구 추가" @click="showAddFriendModal = true">➕</button>
          </div>

          <!-- My Profile Box -->
          <div class="my-profile-card" @click="activeProfileUser = currentUser">
            <div class="avatar-circle" :style="{ background: currentUser.avatar_color || '#06C755' }">
              {{ currentUser.display_name.slice(0, 1) }}
            </div>
            <div class="profile-info">
              <strong class="name">{{ currentUser.display_name }}</strong>
              <small class="status-msg">{{ currentUser.status_message || '상태메시지를 입력하세요' }}</small>
              <div class="bgm-tag">🎵 Official Hige Dandism - Subtitle</div>
            </div>
          </div>

          <!-- Search input -->
          <div class="search-box">
            <input v-model="searchKeyword" placeholder="친구, 공식 계정 검색" />
          </div>

          <div class="friends-scroll-area">
            <!-- Birthday Friends Section -->
            <div class="friends-category">
              <div class="category-header">🎂 오늘 생일인 친구</div>
              <div class="friend-row">
                <div class="avatar-circle sm" style="background:#f97316">A</div>
                <div class="info">
                  <strong>Alice Kim</strong>
                  <small class="muted">오늘 생일입니다 🎉</small>
                </div>
                <button class="btn-gift" @click="openSendMoneyModal({ id: '00000000-0000-0000-0000-000000000003', display_name: 'Alice Kim' })">
                  선물하기
                </button>
              </div>
            </div>

            <!-- Official Accounts Section -->
            <div class="friends-category">
              <div class="category-header">🟢 공식 계정 ({{ officialAccounts.length }})</div>
              <div v-for="oa in officialAccounts" :key="oa.id" class="friend-row" @click="startDirectChat({ id: oa.id, display_name: oa.name, official: true })">
                <div class="avatar-circle sm" :style="{ background: oa.color || '#06C755' }">
                  {{ oa.name.slice(0, 1) }}
                </div>
                <div class="info">
                  <strong>{{ oa.name }} <span class="badge-verified">✓</span></strong>
                  <small class="muted">{{ oa.description }}</small>
                </div>
                <button class="btn-chat-shortcut">대화</button>
              </div>
            </div>

            <!-- Friends List Section -->
            <div class="friends-category">
              <div class="category-header">👥 친구 ({{ friends.length }})</div>
              <div v-for="f in friends" :key="f.id" class="friend-row">
                <div class="avatar-circle sm" :style="{ background: f.avatar_color || '#0ea5e9' }" @click="activeProfileUser = f">
                  {{ f.display_name.slice(0, 1) }}
                </div>
                <div class="info" @click="activeProfileUser = f">
                  <strong>{{ f.display_name }}</strong>
                  <small class="muted">{{ f.status_message || f.bio || 'LINE 친구' }}</small>
                </div>
                <div class="row-actions">
                  <button class="btn-chat-shortcut" @click="startDirectChat(f)">1:1 대화</button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- TAB 2: CHATS (Conversation List) -->
        <div v-else-if="activeNav === 'chats'" class="sidebar-content">
          <div class="sidebar-header">
            <h2>대화</h2>
            <div class="header-btns">
              <button class="btn-icon" title="그룹 대화방 만들기" @click="showCreateGroupModal = true">👥+</button>
            </div>
          </div>

          <!-- Chat Filter Pills -->
          <div class="chat-filters">
            <button :class="{ active: chatFilter === 'all' }" @click="chatFilter = 'all'">전체</button>
            <button :class="{ active: chatFilter === 'direct' }" @click="chatFilter = 'direct'">1:1</button>
            <button :class="{ active: chatFilter === 'group' }" @click="chatFilter = 'group'">그룹</button>
            <button :class="{ active: chatFilter === 'unread' }" @click="chatFilter = 'unread'">안읽음</button>
          </div>

          <div class="search-box">
            <input v-model="searchKeyword" placeholder="대화방 또는 메시지 검색" />
          </div>

          <div class="chats-scroll-area">
            <div
              v-for="c in filteredChats"
              :key="c.id"
              class="chat-row"
              :class="{ selected: selectedChatId === c.id }"
              @click="selectChat(c.id)"
            >
              <div class="avatar-circle chat-avatar" :class="{ 'group-avatar': c.kind === 'group' }">
                {{ c.title ? c.title.slice(0, 1) : 'L' }}
              </div>
              <div class="chat-row-info">
                <div class="chat-title-line">
                  <strong class="chat-title">{{ c.title || '대화방' }}</strong>
                  <span v-if="c.kind === 'group'" class="member-count">{{ c.member_count || 2 }}</span>
                  <span v-if="c.pinned" class="badge-pin">📌</span>
                  <small class="chat-time">{{ formatTime(c.updated_at || '') }}</small>
                </div>
                <div class="chat-preview-line">
                  <span class="preview-text">
                    {{ c.last_message?.unsent ? '메시지를 취소했습니다' : (c.last_message?.body || '새로운 대화가 시작되었습니다.') }}
                  </span>
                  <span v-if="c.unread_count && c.unread_count > 0" class="unread-badge">
                    {{ c.unread_count }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- TAB 3: VOOM (Shorts & Story) -->
        <div v-else-if="activeNav === 'voom'" class="sidebar-content">
          <div class="sidebar-header">
            <h2>LINE VOOM</h2>
            <button class="btn-icon" title="새 스토리 올리기" @click="showNewStoryModal = true">📷+</button>
          </div>

          <!-- Stories carousel in VOOM sidebar -->
          <div class="stories-row">
            <div class="story-item my-story" @click="showNewStoryModal = true">
              <div class="story-ring plus">+</div>
              <small>스토리 추가</small>
            </div>
            <div v-for="s in stories" :key="s.id" class="story-item">
              <div class="story-ring" :style="{ background: s.bg }">
                {{ s.author?.display_name?.slice(0, 1) || 'L' }}
              </div>
              <small>{{ s.author?.display_name || '친구' }}</small>
            </div>
          </div>

          <!-- Video feed list -->
          <div class="voom-list">
            <div v-for="v in voomVideos" :key="v.id" class="voom-card">
              <div class="voom-banner" :style="{ background: v.bg }">
                <span class="voom-emoji">{{ v.emoji }}</span>
              </div>
              <div class="voom-meta">
                <strong>{{ v.title }}</strong>
                <div class="voom-actions">
                  <button class="btn-like" :class="{ liked: v.liked }" @click="likeVoomPost(v.id)">
                    ❤️ {{ v.like_count }}
                  </button>
                  <span class="voom-author">by {{ v.author?.display_name }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- TAB 4: OPENCHAT (Communities) -->
        <div v-else-if="activeNav === 'openchat'" class="sidebar-content">
          <div class="sidebar-header">
            <h2>오픈채팅</h2>
          </div>
          <div class="openchat-intro-banner">
            <strong>🌐 관심사 기반 오픈채팅</strong>
            <p>프로필 닉네임을 설정하고 다양한 주제로 자유롭게 대화하세요.</p>
          </div>
          <div class="openchat-list">
            <div class="openchat-card" @click="activeNav = 'chats'">
              <div class="card-icon">💻</div>
              <div class="card-info">
                <strong>LINE Global Developer Lounge</strong>
                <small>클라우드플레어 엣지 및 프론트엔드 엔지니어 오픈채팅</small>
                <span class="badge-members">👥 48명 참여 중</span>
              </div>
            </div>
            <div class="openchat-card">
              <div class="card-icon">☕</div>
              <div class="card-info">
                <strong>서울 카페 & 디저트 투어</strong>
                <small>주말 핫플레이스 탐방 및 사진 공유</small>
                <span class="badge-members">👥 124명 참여 중</span>
              </div>
            </div>
          </div>
        </div>

        <!-- TAB 5: WALLET / PAY & SERVICES -->
        <div v-else-if="activeNav === 'wallet'" class="sidebar-content">
          <div class="sidebar-header">
            <h2>LINE Pay & 더보기</h2>
          </div>

          <!-- LINE Pay Card -->
          <div class="line-pay-card">
            <div class="pay-top">
              <span class="pay-logo">LINE Pay</span>
              <span class="pay-currency">KRW</span>
            </div>
            <div class="pay-balance">₩ {{ walletBalance.toLocaleString() }}</div>
            <div class="pay-actions">
              <button class="btn-pay-action" @click="showChargeModal = true">➕ 충전하기</button>
              <button class="btn-pay-action secondary" @click="showSendMoneyModal = true">💸 송금하기</button>
            </div>
          </div>

          <!-- Services Grid -->
          <div class="services-grid">
            <div class="service-box" @click="showKeepModal = true">
              <span class="icon">💾</span>
              <span class="title">Keep 보관함</span>
            </div>
            <div class="service-box" @click="activeNav = 'voom'">
              <span class="icon">🎬</span>
              <span class="title">LINE VOOM</span>
            </div>
            <div class="service-box" @click="showSettingsModal = true">
              <span class="icon">🎨</span>
              <span class="title">테마 / 설정</span>
            </div>
            <div class="service-box" @click="showStickerPicker = true">
              <span class="icon">🐻</span>
              <span class="title">스티커 샵</span>
            </div>
          </div>

          <!-- Recent Transactions -->
          <div class="tx-history">
            <h3>최근 이용 내역</h3>
            <div v-if="!walletTx.length" class="empty-tx">거래 내역이 없습니다.</div>
            <div v-for="t in walletTx.slice(0, 5)" :key="t.id" class="tx-row">
              <div class="tx-desc">
                <strong>{{ t.note || t.counterparty || 'LINE Pay' }}</strong>
                <small>{{ formatTime(t.created_at) }}</small>
              </div>
              <span class="tx-amount" :class="{ plus: t.amount > 0 }">
                {{ t.amount > 0 ? '+' : '' }}{{ t.amount.toLocaleString() }}원
              </span>
            </div>
          </div>
        </div>
      </aside>

      <!-- (Column 3) Right Main Chat / Conversation Window -->
      <section v-if="selectedChat" class="line-chat-pane">
        <!-- Chat Header -->
        <header class="chat-header">
          <div class="header-left">
            <div class="avatar-circle sm" :style="{ background: selectedChat.kind === 'group' ? '#0ea5e9' : '#06C755' }">
              {{ selectedChat.title ? selectedChat.title.slice(0, 1) : 'L' }}
            </div>
            <div class="chat-info-block">
              <div class="title-row">
                <h3>{{ selectedChat.title }}</h3>
                <span v-if="selectedChat.kind === 'group'" class="member-pill">👥 {{ selectedChat.member_count || 2 }}</span>
                <span v-if="selectedChat.official" class="badge-verified">✓ 공식계정</span>
              </div>
              <small class="e2ee-tag">🔒 Letter Sealing 종단간 암호화 적용됨</small>
            </div>
          </div>

          <div class="header-right-actions">
            <button class="btn-call" title="무료 음성통화" @click="startCall('voice')">📞 음성통화</button>
            <button class="btn-call video" title="영상통화" @click="startCall('video')">📹 영상통화</button>
            <button class="btn-drawer-toggle" :class="{ active: activeDrawer !== 'none' }" @click="toggleDrawer('notes')">
              ☰ 메뉴
            </button>
          </div>
        </header>

        <!-- Pinned Announcement (공지사항) -->
        <div v-if="pinnedList.length" class="pinned-notice-bar" :class="{ expanded: pinnedExpanded }">
          <div class="pin-icon">📌</div>
          <div class="pin-body" @click="pinnedExpanded = !pinnedExpanded">
            <strong>공지: </strong>{{ pinnedList[0].body || '상단 고정된 대화 공지사항입니다.' }}
          </div>
          <div class="pin-controls">
            <button class="btn-pin-toggle" @click="pinnedExpanded = !pinnedExpanded">
              {{ pinnedExpanded ? '▲ 접기' : '▼ 펼치기' }}
            </button>
            <button class="btn-pin-unpin" title="공지 해제" @click="unpinMessage(pinnedList[0].message_id)">✕</button>
          </div>
        </div>

        <!-- Chat Workspace Body: Messages + Drawer -->
        <div class="chat-body-workspace">
          <!-- Messages Scroll Stream -->
          <div class="messages-stream">
            <div class="date-divider">
              <span>2026년 10월 26일 월요일</span>
            </div>

            <!-- Active Poll Card in Stream -->
            <div v-if="activePolls.length" class="inline-poll-card">
              <div class="poll-header">
                <strong>📊 {{ activePolls[0].question }}</strong>
                <span class="poll-status">진행 중</span>
              </div>
              <div class="poll-options">
                <div v-for="opt in activePolls[0].options" :key="opt.id" class="poll-option-row">
                  <div class="option-text-votes">
                    <span>{{ opt.text }}</span>
                    <small>{{ opt.votes }}표</small>
                  </div>
                  <button class="btn-vote" :class="{ voted: opt.voted }" @click="votePoll(activePolls[0].id, opt.id)">
                    {{ opt.voted ? '✓ 투표완료' : '투표' }}
                  </button>
                </div>
              </div>
            </div>

            <!-- Individual Message Bubbles -->
            <div
              v-for="m in messages"
              :key="m.id"
              class="message-row"
              :class="{ mine: m.sender_id === currentUser.id, system: m.kind === 'system' }"
            >
              <!-- System Message -->
              <div v-if="m.kind === 'system'" class="system-bubble">
                {{ m.body }}
              </div>

              <!-- Chat Bubble (Partner or Mine) -->
              <div v-else class="bubble-container">
                <!-- Partner Avatar -->
                <div v-if="m.sender_id !== currentUser.id" class="bubble-avatar">
                  {{ (selectedChat.title || 'U').slice(0, 1) }}
                </div>

                <div class="bubble-wrapper">
                  <!-- Sender Name if partner -->
                  <div v-if="m.sender_id !== currentUser.id" class="bubble-sender-name">
                    {{ selectedChat.kind === 'group' ? '그룹 멤버' : selectedChat.title }}
                  </div>

                  <!-- Reply Preview Quotation -->
                  <div v-if="m.reply_preview" class="reply-quoted-box">
                    <small>↩ 답장: {{ m.reply_preview.body }}</small>
                  </div>

                  <!-- Bubble Body -->
                  <div class="bubble-content" :class="[m.kind, { unsent: m.unsent }]">
                    <!-- Text -->
                    <span v-if="m.kind === 'text'">{{ m.body }}</span>

                    <!-- Sticker -->
                    <div v-else-if="m.kind === 'sticker'" class="sticker-bubble">
                      <span class="big-sticker">{{ m.body }}</span>
                    </div>

                    <!-- Image / Attachment -->
                    <div v-else-if="m.kind === 'image'" class="attachment-bubble">
                      <div class="mock-image-preview">🖼️ {{ m.body }}</div>
                    </div>

                    <!-- Call log bubble -->
                    <div v-else-if="m.kind === 'call'" class="call-log-bubble">
                      📞 통화가 종료되었습니다 ({{ m.metadata?.duration || 0 }}초)
                    </div>

                    <!-- Pay bubble -->
                    <div v-else-if="m.kind === 'pay'" class="pay-transfer-bubble">
                      💸 {{ (m.metadata?.amount || 0).toLocaleString() }}원이 송금되었습니다.
                    </div>
                  </div>

                  <!-- Reactions Badges on Bubble -->
                  <div v-if="m.reactions && Object.keys(m.reactions).length" class="reaction-badge-group">
                    <span v-for="(count, emoji) in m.reactions" :key="emoji" class="reaction-pill">
                      {{ emoji }} {{ count }}
                    </span>
                  </div>

                  <!-- Hover Actions Tray (Reply, Pin, Keep, Copy, Unsend, Reactions) -->
                  <div class="bubble-hover-actions">
                    <button title="하트" @click="reactToMessage(m.id, '❤️')">❤️</button>
                    <button title="좋아요" @click="reactToMessage(m.id, '👍')">👍</button>
                    <button title="답장" @click="startReply(m)">↩</button>
                    <button title="Keep 보관" @click="saveToKeep(m)">💾</button>
                    <button title="공지 등록" @click="pinMessage(m.id)">📌</button>
                    <button title="복사" @click="copyMessageText(m.body)">📋</button>
                    <button v-if="m.sender_id === currentUser.id && !m.unsent" title="전송 취소" @click="unsendMessage(m.id)">🚫</button>
                  </div>
                </div>

                <!-- Timestamp and Read Receipt -->
                <div class="bubble-meta">
                  <span v-if="m.sender_id === currentUser.id" class="read-receipt">읽음</span>
                  <span class="bubble-time">{{ formatTime(m.created_at) }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Optional Drawer Panel (Notes, Albums, Events) -->
          <aside v-if="activeDrawer !== 'none'" class="chat-drawer-pane">
            <div class="drawer-header">
              <div class="drawer-tabs">
                <button :class="{ active: activeDrawer === 'notes' }" @click="toggleDrawer('notes')">노트</button>
                <button :class="{ active: activeDrawer === 'events' }" @click="toggleDrawer('events')">일정</button>
                <button :class="{ active: activeDrawer === 'media' }" @click="toggleDrawer('media')">사진·동영상</button>
              </div>
              <button class="btn-close-drawer" @click="activeDrawer = 'none'">✕</button>
            </div>

            <!-- Notes Tab -->
            <div v-if="activeDrawer === 'notes'" class="drawer-body">
              <div class="drawer-action-bar">
                <strong>대화방 릴레이 노트</strong>
              </div>
              <div v-for="n in chatNotes" :key="n.id" class="drawer-note-card">
                <strong>{{ n.title }}</strong>
                <p>{{ n.body }}</p>
                <small class="muted">작성: {{ n.author?.display_name }}</small>
              </div>
            </div>

            <!-- Events Tab -->
            <div v-else-if="activeDrawer === 'events'" class="drawer-body">
              <div class="drawer-action-bar">
                <button class="btn-primary sm" @click="showEventCreateModal = true">+ 새 일정 잡기</button>
              </div>
              <div v-for="ev in chatEvents" :key="ev.id" class="drawer-event-card">
                <strong>📅 {{ ev.title }}</strong>
                <div class="event-time">일시: {{ new Date(ev.starts_at).toLocaleString('ko-KR') }}</div>
                <div class="event-loc">장소: {{ ev.location || '온라인 미팅' }}</div>
              </div>
            </div>

            <!-- Media Tab -->
            <div v-else-if="activeDrawer === 'media'" class="drawer-body">
              <div class="media-grid">
                <div class="media-thumb">📷 사진 1</div>
                <div class="media-thumb">📷 사진 2</div>
                <div class="media-thumb">📷 사진 3</div>
              </div>
            </div>
          </aside>
        </div>

        <!-- Reply Bar preview -->
        <div v-if="replyToMessage" class="reply-active-bar">
          <div class="reply-info">
            <strong>↩ {{ replyToMessage.sender_id === currentUser.id ? '나' : '상대방' }}에게 답장:</strong>
            <span class="preview">{{ replyToMessage.body }}</span>
          </div>
          <button class="btn-cancel-reply" @click="replyToMessage = null">✕</button>
        </div>

        <!-- Sticker Picker Drawer Popover -->
        <div v-if="showStickerPicker" class="sticker-popover">
          <div class="sticker-tabs">
            <button
              v-for="(pack, idx) in stickersList"
              :key="pack.id"
              :class="{ active: activeStickerTab === idx }"
              @click="activeStickerTab = idx"
            >
              {{ pack.name }}
            </button>
            <button class="btn-close-sticker" @click="showStickerPicker = false">✕</button>
          </div>
          <div class="sticker-palette-grid">
            <button
              v-for="stk in stickersList[activeStickerTab].items"
              :key="stk"
              class="sticker-choice-btn"
              @click="sendSticker(stk)"
            >
              {{ stk }}
            </button>
          </div>
        </div>

        <!-- Chat Input Composer -->
        <footer class="chat-composer">
          <!-- Attachment bar icons -->
          <div class="composer-toolbar">
            <button class="btn-tool" title="사진 전송" @click="sendAttachmentMock('image')">📷</button>
            <button class="btn-tool" title="스티커 열기" @click="showStickerPicker = !showStickerPicker">😊</button>
            <button class="btn-tool" title="음성 메모" @click="sendAttachmentMock('voice')">🎙️</button>
            <button class="btn-tool" title="투표 만들기" @click="showPollCreateModal = true">📊</button>
            <button class="btn-tool" title="일정 등록" @click="showEventCreateModal = true">📅</button>
            <button class="btn-tool" title="파일 첨부" @click="sendAttachmentMock('file')">📎</button>
          </div>

          <form class="composer-input-area" @submit.prevent="sendMessage">
            <textarea
              v-model="draftMessage"
              placeholder="메시지를 입력하세요 (Enter 전송, Shift+Enter 줄바꿈)"
              rows="2"
              @keydown.enter.exact.prevent="sendMessage"
            ></textarea>
            <button class="btn-send-line" :disabled="!draftMessage.trim()">
              전송
            </button>
          </form>
        </footer>
      </section>

      <!-- Empty State View when no chat is selected -->
      <section v-else class="empty-welcome-pane">
        <div class="welcome-box">
          <div class="welcome-logo">LINE</div>
          <h2>LINE 메신저에 오신 것을 환영합니다</h2>
          <p>왼쪽 목록에서 대화방을 선택하거나 친구와 1:1 대화를 시작하세요.</p>
          <div class="quick-welcome-actions">
            <button class="btn-primary" @click="activeNav = 'home'">👥 친구 목록 보기</button>
            <button class="btn-secondary" @click="showCreateGroupModal = true">💬 그룹 대화 만들기</button>
          </div>
        </div>
      </section>
    </main>

    <!-- CALLING OVERLAY MODAL (WebRTC Voice & Video Call) -->
    <div v-if="activeCall" class="call-overlay-modal">
      <div class="call-dialog-box">
        <div class="call-avatar-pulse">
          <div class="avatar-circle lg">{{ activeCall.title.slice(0, 1) }}</div>
        </div>
        <h3>{{ activeCall.title }}</h3>
        <p class="call-type-status">
          {{ activeCall.kind === 'video' ? '📹 영상통화' : '📞 음성통화' }} ·
          <span v-if="activeCall.state === 'ringing'">연결 중...</span>
          <span v-else-if="activeCall.state === 'connected'" class="timer-active">{{ formatDuration(callDuration) }}</span>
        </p>

        <!-- Call Action Buttons -->
        <div class="call-controls-row">
          <template v-if="activeCall.state === 'connected'">
            <button class="btn-call-circle" :class="{ off: isCallMuted }" @click="isCallMuted = !isCallMuted">
              {{ isCallMuted ? '🔇' : '🎤' }}
            </button>
            <button v-if="activeCall.kind === 'video'" class="btn-call-circle" :class="{ off: isCallVideoOff }" @click="isCallVideoOff = !isCallVideoOff">
              {{ isCallVideoOff ? '🚫' : '📹' }}
            </button>
            <button class="btn-call-circle" :class="{ off: !isCallSpeakerOn }" @click="isCallSpeakerOn = !isCallSpeakerOn">
              🔊
            </button>
          </template>
          <button v-if="activeCall.state === 'ringing'" class="btn-call-answer" @click="updateCallState('connected')">
            📞 수락
          </button>
          <button class="btn-call-hangup" @click="updateCallState('ended')">
            🔴 통화 종료
          </button>
        </div>
      </div>
    </div>

    <!-- PROFILE MODAL (Full LINE User Profile View) -->
    <div v-if="activeProfileUser" class="modal-backdrop" @click.self="activeProfileUser = null">
      <div class="profile-modal-card">
        <div class="profile-cover-header" :style="{ background: activeProfileUser.avatar_color || '#06C755' }">
          <button class="btn-modal-close" @click="activeProfileUser = null">✕</button>
        </div>
        <div class="profile-avatar-center">
          <div class="avatar-circle xl">{{ activeProfileUser.display_name.slice(0, 1) }}</div>
        </div>
        <div class="profile-body-center">
          <h2>{{ activeProfileUser.display_name }}</h2>
          <div class="user-handle">@{{ activeProfileUser.handle || 'user' }}</div>
          <p class="user-status-text">"{{ activeProfileUser.status_message || activeProfileUser.bio || '상태메시지가 없습니다.' }}"</p>
        </div>
        <div class="profile-actions-footer">
          <button class="btn-profile-action" @click="startDirectChat(activeProfileUser); activeProfileUser = null">
            💬 1:1 대화
          </button>
          <button class="btn-profile-action" @click="startDirectChat(activeProfileUser); startCall('voice'); activeProfileUser = null">
            📞 무료통화
          </button>
          <button class="btn-profile-action" @click="startDirectChat(activeProfileUser); startCall('video'); activeProfileUser = null">
            📹 영상통화
          </button>
          <button class="btn-profile-action" @click="openSendMoneyModal(activeProfileUser); activeProfileUser = null">
            💸 송금
          </button>
        </div>
      </div>
    </div>

    <!-- KEEP VAULT MODAL -->
    <div v-if="showKeepModal" class="modal-backdrop" @click.self="showKeepModal = false">
      <div class="standard-modal-box keep-modal">
        <div class="modal-header">
          <h3>💾 LINE Keep 보관함 ({{ keepItems.length }})</h3>
          <button class="btn-modal-close" @click="showKeepModal = false">✕</button>
        </div>
        <div class="keep-filter-bar">
          <button :class="{ active: keepFilter === 'all' }" @click="keepFilter = 'all'">전체</button>
          <button :class="{ active: keepFilter === 'text' }" @click="keepFilter = 'text'">텍스트</button>
          <button :class="{ active: keepFilter === 'image' }" @click="keepFilter = 'image'">사진/미디어</button>
          <button :class="{ active: keepFilter === 'link' }" @click="keepFilter = 'link'">링크</button>
        </div>
        <input v-model="keepSearch" placeholder="보관된 내용 검색..." class="keep-search-input" />
        <div class="keep-scroll-content">
          <p v-if="!filteredKeepItems.length" class="empty-muted">저장된 Keep 항목이 없습니다.</p>
          <div v-for="k in filteredKeepItems" :key="k.id || k.message_id" class="keep-item-card">
            <div class="keep-item-header">
              <span class="badge-keep-type">{{ k.kind }}</span>
              <button class="btn-delete-keep" @click="deleteFromKeep(k)">✕ 삭제</button>
            </div>
            <div class="keep-body-text">{{ k.body }}</div>
            <small class="muted">{{ new Date(k.created_at).toLocaleString('ko-KR') }}</small>
          </div>
        </div>
      </div>
    </div>

    <!-- SEND MONEY MODAL -->
    <div v-if="showSendMoneyModal" class="modal-backdrop" @click.self="showSendMoneyModal = false">
      <div class="standard-modal-box">
        <div class="modal-header">
          <h3>💸 LINE Pay 송금</h3>
          <button class="btn-modal-close" @click="showSendMoneyModal = false">✕</button>
        </div>
        <div class="modal-body-form">
          <div v-if="sendTargetFriend" class="send-target-info">
            받는 사람: <strong>{{ sendTargetFriend.display_name }}</strong>
          </div>
          <label>송금 금액 (원)</label>
          <input v-model.number="sendAmountInput" type="number" min="1000" step="1000" />
          <div class="quick-amounts">
            <button @click="sendAmountInput = 10000">+1만</button>
            <button @click="sendAmountInput = 30000">+3만</button>
            <button @click="sendAmountInput = 50000">+5만</button>
            <button @click="sendAmountInput = 100000">+10만</button>
          </div>
          <label>보내는 메모</label>
          <input v-model="sendMemoInput" placeholder="메모 입력" />
          <div class="modal-actions-right">
            <button class="btn-secondary" @click="showSendMoneyModal = false">취소</button>
            <button class="btn-primary" @click="executeSendMoney">송금하기</button>
          </div>
        </div>
      </div>
    </div>

    <!-- CHARGE MONEY MODAL -->
    <div v-if="showChargeModal" class="modal-backdrop" @click.self="showChargeModal = false">
      <div class="standard-modal-box">
        <div class="modal-header">
          <h3>➕ LINE Pay 잔액 충전</h3>
          <button class="btn-modal-close" @click="showChargeModal = false">✕</button>
        </div>
        <div class="modal-body-form">
          <p>충전할 금액을 선택하세요:</p>
          <div class="charge-options-grid">
            <button class="btn-charge-option" @click="executeChargeMoney(10000)">+ 10,000원</button>
            <button class="btn-charge-option" @click="executeChargeMoney(30000)">+ 30,000원</button>
            <button class="btn-charge-option" @click="executeChargeMoney(50000)">+ 50,000원</button>
            <button class="btn-charge-option" @click="executeChargeMoney(100000)">+ 100,000원</button>
          </div>
        </div>
      </div>
    </div>

    <!-- CREATE POLL MODAL -->
    <div v-if="showPollCreateModal" class="modal-backdrop" @click.self="showPollCreateModal = false">
      <div class="standard-modal-box">
        <div class="modal-header">
          <h3>📊 대화방 새 투표 만들기</h3>
          <button class="btn-modal-close" @click="showPollCreateModal = false">✕</button>
        </div>
        <div class="modal-body-form">
          <label>질문</label>
          <input v-model="newPollQuestion" placeholder="예: 오늘 점심 메뉴는?" />
          <label>선택지 1</label>
          <input v-model="newPollOption1" placeholder="선택지 1" />
          <label>선택지 2</label>
          <input v-model="newPollOption2" placeholder="선택지 2" />
          <label>선택지 3 (선택)</label>
          <input v-model="newPollOption3" placeholder="선택지 3" />
          <div class="modal-actions-right">
            <button class="btn-secondary" @click="showPollCreateModal = false">취소</button>
            <button class="btn-primary" @click="createPoll">투표 등록</button>
          </div>
        </div>
      </div>
    </div>

    <!-- CREATE EVENT MODAL -->
    <div v-if="showEventCreateModal" class="modal-backdrop" @click.self="showEventCreateModal = false">
      <div class="standard-modal-box">
        <div class="modal-header">
          <h3>📅 대화방 일정 잡기</h3>
          <button class="btn-modal-close" @click="showEventCreateModal = false">✕</button>
        </div>
        <div class="modal-body-form">
          <label>일정 제목</label>
          <input v-model="newEventTitle" placeholder="예: 팀 주간 싱크 미팅" />
          <label>날짜 및 시간</label>
          <input v-model="newEventDate" type="datetime-local" />
          <label>장소 / 링크</label>
          <input v-model="newEventLocation" placeholder="예: 구글 미트 / 강남역" />
          <div class="modal-actions-right">
            <button class="btn-secondary" @click="showEventCreateModal = false">취소</button>
            <button class="btn-primary" @click="createEvent">일정 등록</button>
          </div>
        </div>
      </div>
    </div>

    <!-- SETTINGS & THEMES MODAL -->
    <div v-if="showSettingsModal" class="modal-backdrop" @click.self="showSettingsModal = false">
      <div class="standard-modal-box">
        <div class="modal-header">
          <h3>⚙️ 라인 환경설정 & 테마</h3>
          <button class="btn-modal-close" @click="showSettingsModal = false">✕</button>
        </div>
        <div class="modal-body-form">
          <div class="setting-group">
            <strong>🎨 디자인 테마 선택</strong>
            <div class="theme-choice-buttons">
              <button class="btn-theme-swatch mint" :class="{ active: activeTheme === 'mint' }" @click="switchTheme('mint')">
                🌱 민트 (LINE 기본)
              </button>
              <button class="btn-theme-swatch ocean" :class="{ active: activeTheme === 'ocean' }" @click="switchTheme('ocean')">
                🌊 오션 블루
              </button>
              <button class="btn-theme-swatch sunset" :class="{ active: activeTheme === 'sunset' }" @click="switchTheme('sunset')">
                🌅 선셋 글로우
              </button>
              <button class="btn-theme-swatch sakura" :class="{ active: activeTheme === 'sakura' }" @click="switchTheme('sakura')">
                🌸 사쿠라 핑크
              </button>
              <button class="btn-theme-swatch midnight" :class="{ active: activeTheme === 'midnight' }" @click="switchTheme('midnight')">
                🌙 미드나잇 다크
              </button>
            </div>
          </div>

          <div class="setting-toggle-row">
            <span>🔒 Letter Sealing 종단간 암호화</span>
            <input type="checkbox" checked disabled />
          </div>
          <div class="setting-toggle-row">
            <span>👀 읽음 확인 표시 (Read Receipts)</span>
            <input type="checkbox" checked />
          </div>
          <div class="setting-toggle-row">
            <span>🔔 메시지 수신 사운드 알림</span>
            <input type="checkbox" checked />
          </div>

          <div class="modal-actions-right">
            <button class="btn-primary" @click="showSettingsModal = false">완료</button>
          </div>
        </div>
      </div>
    </div>

    <!-- CREATE GROUP MODAL -->
    <div v-if="showCreateGroupModal" class="modal-backdrop" @click.self="showCreateGroupModal = false">
      <div class="standard-modal-box">
        <div class="modal-header">
          <h3>👥 새 그룹 채팅방 만들기</h3>
          <button class="btn-modal-close" @click="showCreateGroupModal = false">✕</button>
        </div>
        <div class="modal-body-form">
          <label>그룹 이름</label>
          <input v-model="groupTitleInput" placeholder="그룹 채팅방 이름을 입력하세요" />
          <label>초대할 친구 선택 ({{ selectedFriendsForGroup.length }}명 선택됨)</label>
          <div class="select-friends-scroll">
            <label v-for="f in friends" :key="f.id" class="friend-check-item">
              <input v-model="selectedFriendsForGroup" type="checkbox" :value="f.id" />
              <span>{{ f.display_name }}</span>
            </label>
          </div>
          <div class="modal-actions-right">
            <button class="btn-secondary" @click="showCreateGroupModal = false">취소</button>
            <button class="btn-primary" :disabled="!groupTitleInput.trim() || !selectedFriendsForGroup.length" @click="createGroupChat">
              그룹 생성
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ADD FRIEND MODAL -->
    <div v-if="showAddFriendModal" class="modal-backdrop" @click.self="showAddFriendModal = false">
      <div class="standard-modal-box">
        <div class="modal-header">
          <h3>➕ 친구 추가</h3>
          <button class="btn-modal-close" @click="showAddFriendModal = false">✕</button>
        </div>
        <div class="modal-body-form">
          <label>라인 아이디(ID)로 검색</label>
          <div class="flex-row">
            <input v-model="addFriendHandleInput" placeholder="예: alice_kim" />
            <button class="btn-primary" @click="addFriendByHandle">추가</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* ==========================================================================
   LINE OFFICIAL SIGNATURE DESIGN SYSTEM & MODERN UI/UX
   ========================================================================== */
:root {
  --line-green: #06C755;
  --line-green-hover: #05b34c;
  --line-green-light: #e8f8ee;
  --line-bubble-mine: #06C755;
  --line-bubble-mine-text: #ffffff;
  --line-bubble-other: #ffffff;
  --line-bubble-other-text: #191919;
  --line-bg: #849ebf;
  --line-panel-bg: #ffffff;
  --line-text-primary: #191919;
  --line-text-muted: #64748b;
  --line-border: #e2e8f0;
}

.theme-mint {
  --line-green: #06C755;
  --line-bg: #9bbad8;
}
.theme-ocean {
  --line-green: #0ea5e9;
  --line-bg: #8faec9;
}
.theme-sunset {
  --line-green: #f97316;
  --line-bg: #c99f8f;
}
.theme-sakura {
  --line-green: #ec4899;
  --line-bg: #d4a5b8;
}
.theme-midnight {
  --line-green: #10b981;
  --line-bg: #111827;
  --line-panel-bg: #1f2937;
  --line-text-primary: #f3f4f6;
  --line-text-muted: #9ca3af;
  --line-border: #374151;
  --line-bubble-other: #374151;
  --line-bubble-other-text: #f3f4f6;
}

.line-app {
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
  font-family: -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", "Malgun Gothic", "Noto Sans KR", sans-serif;
  color: var(--line-text-primary);
  background: var(--line-bg);
}

/* --- Global Header --- */
.line-header {
  height: 48px;
  background: #ffffff;
  border-bottom: 1px solid #e5e7eb;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  z-index: 10;
  box-shadow: 0 1px 2px rgba(0,0,0,0.03);
}
.brand {
  display: flex;
  align-items: center;
  gap: 8px;
}
.line-badge-icon {
  background: #06C755;
  color: white;
  font-weight: 900;
  font-size: 11px;
  padding: 3px 6px;
  border-radius: 4px;
  letter-spacing: 0.5px;
}
.brand-title {
  font-weight: 700;
  font-size: 14px;
  color: #111827;
}
.version-badge {
  font-size: 11px;
  background: #f1f5f9;
  color: #64748b;
  padding: 2px 6px;
  border-radius: 10px;
}
.header-consoles {
  display: flex;
  align-items: center;
  gap: 12px;
}
.console-link {
  font-size: 12px;
  font-weight: 600;
  color: #475569;
  text-decoration: none;
  padding: 4px 8px;
  border-radius: 6px;
}
.console-link:hover {
  background: #f1f5f9;
  color: #06C755;
}
.btn-logout {
  font-size: 11px;
  padding: 4px 8px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  background: white;
  cursor: pointer;
}

/* --- Alerts --- */
.alert-banner {
  padding: 8px 16px;
  font-size: 13px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.alert-banner.error { background: #fef2f2; color: #dc2626; border-bottom: 1px solid #fecaca; }
.alert-banner.note { background: #f0fdf4; color: #16a34a; border-bottom: 1px solid #bbf7d0; }
.btn-dismiss { background: none; border: none; cursor: pointer; font-size: 12px; }

/* --- Auth Screen --- */
.auth-container {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #eef2f6;
  padding: 20px;
}
.auth-card {
  width: 100%;
  max-width: 420px;
  background: white;
  border-radius: 16px;
  padding: 32px;
  box-shadow: 0 10px 25px -5px rgba(0,0,0,0.1);
}
.auth-logo-area {
  text-align: center;
  margin-bottom: 24px;
}
.line-big-logo {
  display: inline-block;
  background: #06C755;
  color: white;
  font-weight: 900;
  font-size: 24px;
  padding: 8px 18px;
  border-radius: 12px;
  margin-bottom: 12px;
}
.auth-card h1 {
  margin: 0;
  font-size: 22px;
  font-weight: 800;
  color: #111827;
}
.auth-subtitle {
  font-size: 13px;
  color: #64748b;
  margin: 6px 0 0;
}
.auth-demo-box {
  background: #f8fafc;
  border: 1px dashed #cbd5e1;
  border-radius: 10px;
  padding: 12px;
  margin-bottom: 20px;
}
.demo-title {
  font-size: 11px;
  font-weight: 700;
  color: #475569;
  margin-bottom: 8px;
}
.demo-buttons {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.btn-demo {
  text-align: left;
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 600;
  border: 1px solid #e2e8f0;
  background: white;
  border-radius: 8px;
  cursor: pointer;
}
.btn-demo:hover {
  background: #e8f8ee;
  border-color: #06C755;
}
.auth-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.form-group label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  color: #334155;
  margin-bottom: 4px;
}
.form-group input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  font-size: 14px;
  box-sizing: border-box;
}
.line-btn-submit {
  background: #06C755;
  color: white;
  border: none;
  border-radius: 8px;
  padding: 12px;
  font-size: 15px;
  font-weight: 700;
  cursor: pointer;
  margin-top: 6px;
}
.line-btn-submit:hover {
  background: #05b34c;
}
.auth-footer-actions {
  text-align: center;
  margin-top: 16px;
}
.btn-link {
  background: none;
  border: none;
  font-size: 13px;
  color: #06C755;
  cursor: pointer;
  text-decoration: underline;
}

/* --- Messenger Shell 3-Pane Layout --- */
.messenger-shell {
  flex: 1;
  display: flex;
  overflow: hidden;
  height: calc(100vh - 48px);
}

/* (Col 1) Left Dock */
.line-dock {
  width: 68px;
  background: #1e293b;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  flex-shrink: 0;
  z-index: 5;
}
.dock-top, .dock-bottom {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  width: 100%;
}
.dock-app-icon {
  background: #06C755;
  color: white;
  font-weight: 900;
  font-size: 16px;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  display: grid;
  place-items: center;
  margin-bottom: 8px;
}
.dock-btn {
  background: none;
  border: none;
  color: #94a3b8;
  width: 52px;
  height: 52px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  position: relative;
  transition: all 0.15s;
}
.dock-btn .icon { font-size: 20px; }
.dock-btn .label { font-size: 10px; font-weight: 600; margin-top: 2px; }
.dock-btn:hover { background: rgba(255,255,255,0.08); color: #f1f5f9; }
.dock-btn.active { background: #06C755; color: white; }
.dock-badge {
  position: absolute;
  top: 4px;
  right: 6px;
  background: #ef4444;
  color: white;
  font-size: 10px;
  font-weight: 800;
  padding: 2px 5px;
  border-radius: 10px;
}

/* (Col 2) Second Sidebar List */
.line-sidebar {
  width: 320px;
  background: var(--line-panel-bg);
  border-right: 1px solid var(--line-border);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}
.sidebar-content {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.sidebar-header {
  padding: 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.sidebar-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 800;
}
.btn-icon {
  background: none;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  padding: 6px 10px;
  font-size: 13px;
  cursor: pointer;
}
.my-profile-card {
  padding: 12px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  background: #f8fafc;
  border-bottom: 1px solid #f1f5f9;
  cursor: pointer;
}
.avatar-circle {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  color: white;
  font-weight: 800;
  font-size: 16px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
}
.avatar-circle.sm { width: 36px; height: 36px; font-size: 14px; }
.avatar-circle.lg { width: 72px; height: 72px; font-size: 28px; }
.avatar-circle.xl { width: 88px; height: 88px; font-size: 34px; border: 4px solid white; }
.profile-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.profile-info .name { font-size: 14px; font-weight: 700; }
.profile-info .status-msg { font-size: 12px; color: #64748b; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.bgm-tag { font-size: 10px; color: #06C755; margin-top: 2px; }
.search-box {
  padding: 8px 16px;
}
.search-box input {
  width: 100%;
  padding: 8px 12px;
  background: #f1f5f9;
  border: none;
  border-radius: 8px;
  font-size: 13px;
  box-sizing: border-box;
}
.friends-scroll-area, .chats-scroll-area {
  flex: 1;
  overflow-y: auto;
}
.friends-category {
  padding: 8px 0;
}
.category-header {
  font-size: 11px;
  font-weight: 700;
  color: #94a3b8;
  padding: 4px 16px;
}
.friend-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  cursor: pointer;
  transition: background 0.1s;
}
.friend-row:hover { background: #f8fafc; }
.friend-row .info { flex: 1; min-width: 0; }
.friend-row .info strong { display: block; font-size: 13px; }
.badge-verified { color: #06C755; font-size: 11px; }
.btn-gift, .btn-chat-shortcut {
  font-size: 11px;
  padding: 4px 8px;
  border-radius: 6px;
  border: 1px solid #cbd5e1;
  background: white;
  cursor: pointer;
}
.btn-gift { color: #ea580c; border-color: #fdba74; }

/* Chats List */
.chat-filters {
  display: flex;
  gap: 6px;
  padding: 0 16px 8px;
}
.chat-filters button {
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 12px;
  border: none;
  background: #f1f5f9;
  color: #64748b;
  cursor: pointer;
}
.chat-filters button.active { background: #06C755; color: white; }
.chat-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  cursor: pointer;
  border-bottom: 1px solid rgba(0,0,0,0.03);
}
.chat-row:hover { background: #f8fafc; }
.chat-row.selected { background: #e8f8ee; border-left: 4px solid #06C755; }
.chat-row-info { flex: 1; min-width: 0; }
.chat-title-line { display: flex; align-items: center; justify-content: space-between; }
.chat-title { font-size: 14px; font-weight: 700; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.member-count { font-size: 11px; color: #94a3b8; margin-left: 4px; }
.chat-time { font-size: 11px; color: #94a3b8; }
.chat-preview-line { display: flex; justify-content: space-between; align-items: center; margin-top: 4px; }
.preview-text { font-size: 12px; color: #64748b; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 180px; }
.unread-badge { background: #06C755; color: white; font-size: 11px; font-weight: 800; padding: 2px 7px; border-radius: 10px; }

/* VOOM tab */
.stories-row { display: flex; gap: 10px; padding: 12px 16px; overflow-x: auto; border-bottom: 1px solid #f1f5f9; }
.story-item { display: flex; flex-direction: column; align-items: center; gap: 4px; cursor: pointer; }
.story-ring { width: 48px; height: 48px; border-radius: 50%; border: 2px solid #06C755; display: grid; place-items: center; color: white; font-weight: 800; }
.story-ring.plus { border-color: #cbd5e1; color: #06C755; font-size: 20px; }
.voom-list { padding: 12px 16px; display: flex; flex-direction: column; gap: 12px; overflow-y: auto; }
.voom-card { border: 1px solid #e2e8f0; border-radius: 12px; overflow: hidden; background: white; }
.voom-banner { height: 100px; display: grid; place-items: center; font-size: 36px; }
.voom-meta { padding: 10px; }
.voom-actions { display: flex; justify-content: space-between; align-items: center; margin-top: 6px; }
.btn-like { background: none; border: 1px solid #e2e8f0; border-radius: 12px; padding: 4px 8px; font-size: 12px; cursor: pointer; }
.btn-like.liked { background: #fee2e2; border-color: #fca5a5; }

/* OpenChat tab */
.openchat-intro-banner { padding: 12px 16px; background: #f0fdf4; margin: 0 16px 12px; border-radius: 10px; }
.openchat-list { padding: 0 16px; display: flex; flex-direction: column; gap: 10px; }
.openchat-card { display: flex; gap: 12px; padding: 12px; border: 1px solid #e2e8f0; border-radius: 12px; cursor: pointer; }
.openchat-card:hover { background: #f8fafc; border-color: #06C755; }
.card-icon { font-size: 24px; }
.badge-members { display: block; font-size: 11px; color: #06C755; font-weight: 700; margin-top: 4px; }

/* Wallet tab */
.line-pay-card { margin: 16px; background: linear-gradient(135deg, #06C755, #0ea5e9); color: white; padding: 18px; border-radius: 16px; }
.pay-top { display: flex; justify-content: space-between; font-weight: 800; }
.pay-balance { font-size: 24px; font-weight: 900; margin: 12px 0; }
.pay-actions { display: flex; gap: 8px; }
.btn-pay-action { flex: 1; padding: 8px; background: white; color: #06C755; font-weight: 700; border: none; border-radius: 8px; cursor: pointer; font-size: 13px; }
.btn-pay-action.secondary { background: rgba(255,255,255,0.2); color: white; }
.services-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; padding: 0 16px 16px; }
.service-box { background: #f8fafc; padding: 14px; border-radius: 12px; display: flex; flex-direction: column; align-items: center; gap: 6px; cursor: pointer; }
.service-box:hover { background: #e8f8ee; }
.service-box .icon { font-size: 22px; }
.service-box .title { font-size: 12px; font-weight: 700; }
.tx-history { padding: 0 16px; }
.tx-history h3 { font-size: 14px; margin-bottom: 8px; }
.tx-row { display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid #f1f5f9; }
.tx-amount { font-weight: 700; font-size: 13px; }
.tx-amount.plus { color: #06C755; }

/* (Col 3) Right Main Chat Window */
.line-chat-pane {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: var(--line-bg);
  height: 100%;
  position: relative;
}
.chat-header {
  height: 60px;
  background: rgba(255,255,255,0.95);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid rgba(0,0,0,0.08);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  flex-shrink: 0;
}
.header-left { display: flex; align-items: center; gap: 12px; }
.chat-info-block .title-row { display: flex; align-items: center; gap: 8px; }
.chat-info-block h3 { margin: 0; font-size: 16px; font-weight: 800; }
.member-pill { font-size: 11px; background: #f1f5f9; padding: 2px 6px; border-radius: 8px; color: #64748b; }
.e2ee-tag { font-size: 11px; color: #16a34a; font-weight: 600; }
.header-right-actions { display: flex; gap: 8px; align-items: center; }
.btn-call { background: #f1f5f9; border: 1px solid #cbd5e1; border-radius: 8px; padding: 6px 12px; font-size: 12px; font-weight: 600; cursor: pointer; }
.btn-call.video { background: #e8f8ee; color: #06C755; border-color: #06C755; }
.btn-drawer-toggle { background: none; border: 1px solid #cbd5e1; border-radius: 8px; padding: 6px 12px; font-size: 12px; font-weight: 600; cursor: pointer; }
.btn-drawer-toggle.active { background: #1e293b; color: white; }

/* Pinned Announcement */
.pinned-notice-bar {
  background: #ffffff;
  border-bottom: 1px solid #e2e8f0;
  padding: 10px 16px;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.04);
}
.pin-icon { font-size: 16px; }
.pin-body { flex: 1; cursor: pointer; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.pinned-notice-bar.expanded .pin-body { white-space: normal; }
.btn-pin-toggle, .btn-pin-unpin { background: none; border: none; font-size: 11px; color: #64748b; cursor: pointer; }

/* Chat Workspace Body (Stream + Drawer) */
.chat-body-workspace { flex: 1; display: flex; overflow: hidden; position: relative; }
.messages-stream { flex: 1; overflow-y: auto; padding: 20px; display: flex; flex-direction: column; gap: 14px; }
.date-divider { text-align: center; margin: 10px 0; }
.date-divider span { background: rgba(0,0,0,0.15); color: white; font-size: 11px; font-weight: 600; padding: 4px 12px; border-radius: 12px; }

/* Inline Poll Card */
.inline-poll-card { background: white; border-radius: 14px; padding: 14px; margin-bottom: 10px; box-shadow: 0 2px 6px rgba(0,0,0,0.06); }
.poll-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.poll-status { font-size: 11px; background: #e8f8ee; color: #06C755; padding: 2px 6px; border-radius: 4px; font-weight: 700; }
.poll-option-row { display: flex; justify-content: space-between; align-items: center; padding: 8px 10px; background: #f8fafc; border-radius: 8px; margin-bottom: 6px; }
.btn-vote { padding: 4px 10px; font-size: 11px; background: #06C755; color: white; border: none; border-radius: 6px; cursor: pointer; }
.btn-vote.voted { background: #94a3b8; }

/* Messages and Bubbles */
.message-row { display: flex; flex-direction: column; }
.system-bubble { align-self: center; background: rgba(0,0,0,0.2); color: white; font-size: 11px; padding: 4px 12px; border-radius: 12px; }
.bubble-container { display: flex; align-items: flex-end; gap: 8px; max-width: 75%; position: relative; }
.message-row.mine .bubble-container { align-self: flex-end; flex-direction: row-reverse; }
.bubble-avatar { width: 34px; height: 34px; border-radius: 50%; background: #0ea5e9; color: white; font-weight: 700; font-size: 12px; display: grid; place-items: center; align-self: flex-start; }
.bubble-wrapper { position: relative; display: flex; flex-direction: column; }
.bubble-sender-name { font-size: 11px; color: rgba(255,255,255,0.9); margin-bottom: 4px; font-weight: 600; }
.reply-quoted-box { background: rgba(0,0,0,0.05); padding: 4px 8px; border-radius: 6px; font-size: 11px; margin-bottom: 4px; }
.bubble-content {
  background: var(--line-bubble-other);
  color: var(--line-bubble-other-text);
  padding: 10px 14px;
  border-radius: 16px;
  font-size: 14px;
  line-height: 1.45;
  word-break: break-word;
  box-shadow: 0 1px 3px rgba(0,0,0,0.06);
}
.message-row.mine .bubble-content {
  background: var(--line-bubble-mine);
  color: var(--line-bubble-mine-text);
  border-bottom-right-radius: 4px;
}
.message-row:not(.mine) .bubble-content {
  border-bottom-left-radius: 4px;
}
.bubble-content.sticker { background: transparent; box-shadow: none; padding: 0; }
.big-sticker { font-size: 48px; }
.bubble-content.unsent { font-style: italic; opacity: 0.8; }
.reaction-badge-group { display: flex; gap: 4px; margin-top: 4px; }
.reaction-pill { background: white; border: 1px solid #e2e8f0; font-size: 11px; padding: 2px 6px; border-radius: 10px; box-shadow: 0 1px 2px rgba(0,0,0,0.05); }

/* Hover Actions */
.bubble-hover-actions {
  display: none;
  position: absolute;
  top: -24px;
  background: white;
  border-radius: 14px;
  padding: 2px 6px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.15);
  gap: 2px;
  z-index: 5;
}
.bubble-wrapper:hover .bubble-hover-actions { display: flex; }
.message-row.mine .bubble-hover-actions { right: 0; }
.message-row:not(.mine) .bubble-hover-actions { left: 0; }
.bubble-hover-actions button { background: none; border: none; font-size: 12px; padding: 2px 4px; cursor: pointer; border-radius: 4px; }
.bubble-hover-actions button:hover { background: #f1f5f9; }

.bubble-meta { display: flex; flex-direction: column; align-items: flex-end; font-size: 10px; color: rgba(255,255,255,0.85); flex-shrink: 0; }
.message-row.mine .bubble-meta { align-items: flex-end; }
.read-receipt { color: #06C755; font-weight: 700; background: white; padding: 1px 4px; border-radius: 4px; margin-bottom: 2px; font-size: 9px; }

/* Drawer Pane */
.chat-drawer-pane { width: 280px; background: white; border-left: 1px solid #e2e8f0; display: flex; flex-direction: column; }
.drawer-header { padding: 12px; display: flex; justify-content: space-between; border-bottom: 1px solid #f1f5f9; }
.drawer-tabs { display: flex; gap: 6px; }
.drawer-tabs button { padding: 4px 8px; font-size: 12px; border: none; background: #f1f5f9; border-radius: 6px; cursor: pointer; }
.drawer-tabs button.active { background: #06C755; color: white; }
.btn-close-drawer { background: none; border: none; font-size: 13px; cursor: pointer; }
.drawer-body { padding: 14px; overflow-y: auto; flex: 1; }
.drawer-note-card, .drawer-event-card { background: #f8fafc; border-radius: 10px; padding: 10px; margin-bottom: 10px; }

/* Composer */
.reply-active-bar { background: #f1f5f9; padding: 6px 16px; display: flex; justify-content: space-between; align-items: center; font-size: 12px; }
.btn-cancel-reply { background: none; border: none; cursor: pointer; }
.sticker-popover { background: white; border-top: 1px solid #e2e8f0; padding: 10px 16px; height: 160px; overflow-y: auto; }
.sticker-tabs { display: flex; gap: 8px; margin-bottom: 10px; }
.sticker-tabs button { font-size: 12px; padding: 4px 8px; border: 1px solid #e2e8f0; border-radius: 6px; background: white; cursor: pointer; }
.sticker-tabs button.active { background: #06C755; color: white; border-color: #06C755; }
.sticker-palette-grid { display: flex; gap: 12px; flex-wrap: wrap; }
.sticker-choice-btn { font-size: 30px; background: none; border: none; cursor: pointer; }
.chat-composer { background: white; border-top: 1px solid #e2e8f0; padding: 10px 16px; }
.composer-toolbar { display: flex; gap: 12px; margin-bottom: 6px; }
.btn-tool { background: none; border: none; font-size: 18px; cursor: pointer; }
.composer-input-area { display: flex; gap: 10px; align-items: flex-end; }
.composer-input-area textarea {
  flex: 1;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 8px 12px;
  font-family: inherit;
  font-size: 14px;
  resize: none;
  box-sizing: border-box;
}
.btn-send-line {
  background: #06C755;
  color: white;
  border: none;
  border-radius: 8px;
  padding: 10px 18px;
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
}
.btn-send-line:disabled { background: #cbd5e1; cursor: not-allowed; }

/* Empty Welcome Pane */
.empty-welcome-pane { flex: 1; display: grid; place-items: center; background: #f8fafc; }
.welcome-box { text-align: center; max-width: 420px; padding: 30px; }
.welcome-logo { display: inline-block; background: #06C755; color: white; font-weight: 900; font-size: 28px; padding: 10px 22px; border-radius: 14px; margin-bottom: 14px; }
.quick-welcome-actions { display: flex; gap: 10px; justify-content: center; margin-top: 20px; }
.btn-primary { background: #06C755; color: white; border: none; border-radius: 8px; padding: 10px 16px; font-weight: 700; cursor: pointer; }
.btn-secondary { background: white; border: 1px solid #cbd5e1; border-radius: 8px; padding: 10px 16px; font-weight: 700; cursor: pointer; }

/* Calling Overlay Modal */
.call-overlay-modal {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.85);
  backdrop-filter: blur(12px);
  display: grid;
  place-items: center;
  z-index: 100;
}
.call-dialog-box {
  background: #1e293b;
  color: white;
  width: 340px;
  padding: 36px 24px;
  border-radius: 24px;
  text-align: center;
  box-shadow: 0 25px 50px -12px rgba(0,0,0,0.5);
}
.call-avatar-pulse { margin-bottom: 16px; display: inline-block; position: relative; }
.call-type-status { font-size: 14px; color: #94a3b8; margin: 8px 0 24px; }
.timer-active { color: #06C755; font-weight: 700; }
.call-controls-row { display: flex; gap: 12px; justify-content: center; align-items: center; }
.btn-call-circle { width: 44px; height: 44px; border-radius: 50%; background: #334155; border: none; font-size: 18px; color: white; cursor: pointer; }
.btn-call-circle.off { background: #ef4444; }
.btn-call-answer { background: #06C755; color: white; border: none; border-radius: 24px; padding: 12px 24px; font-weight: 800; cursor: pointer; }
.btn-call-hangup { background: #ef4444; color: white; border: none; border-radius: 24px; padding: 12px 24px; font-weight: 800; cursor: pointer; }

/* Modals */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.5);
  display: grid;
  place-items: center;
  z-index: 50;
  padding: 20px;
}
.standard-modal-box {
  background: white;
  border-radius: 16px;
  width: 100%;
  max-width: 440px;
  overflow: hidden;
  box-shadow: 0 20px 25px -5px rgba(0,0,0,0.2);
}
.modal-header {
  padding: 16px 20px;
  border-bottom: 1px solid #e2e8f0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.modal-header h3 { margin: 0; font-size: 16px; font-weight: 800; }
.btn-modal-close { background: none; border: none; font-size: 14px; cursor: pointer; }
.modal-body-form { padding: 20px; display: flex; flex-direction: column; gap: 12px; }
.modal-body-form label { font-size: 12px; font-weight: 700; color: #475569; }
.modal-body-form input { padding: 10px 12px; border: 1px solid #cbd5e1; border-radius: 8px; font-size: 14px; }
.modal-actions-right { display: flex; justify-content: flex-end; gap: 8px; margin-top: 10px; }

/* Profile Card Modal */
.profile-modal-card { width: 340px; background: white; border-radius: 20px; overflow: hidden; box-shadow: 0 25px 50px -12px rgba(0,0,0,0.25); text-align: center; }
.profile-cover-header { height: 120px; position: relative; }
.profile-avatar-center { margin-top: -44px; display: flex; justify-content: center; }
.profile-body-center { padding: 16px 20px; }
.profile-body-center h2 { margin: 0; font-size: 20px; font-weight: 800; }
.user-handle { font-size: 13px; color: #64748b; margin-top: 2px; }
.user-status-text { font-size: 13px; color: #334155; margin: 12px 0 0; font-style: italic; }
.profile-actions-footer { display: flex; border-top: 1px solid #f1f5f9; padding: 12px; justify-content: space-around; }
.btn-profile-action { background: none; border: none; font-size: 12px; font-weight: 700; color: #06C755; cursor: pointer; }

/* Keep Modal */
.keep-modal { max-width: 520px; }
.keep-filter-bar { display: flex; gap: 8px; padding: 12px 20px 0; }
.keep-filter-bar button { padding: 4px 10px; border-radius: 12px; border: none; background: #f1f5f9; font-size: 12px; cursor: pointer; }
.keep-filter-bar button.active { background: #06C755; color: white; }
.keep-search-input { margin: 10px 20px; padding: 8px 12px; border: 1px solid #e2e8f0; border-radius: 8px; font-size: 13px; }
.keep-scroll-content { padding: 0 20px 20px; max-height: 380px; overflow-y: auto; display: flex; flex-direction: column; gap: 10px; }
.keep-item-card { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 10px; padding: 12px; }
.keep-item-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.badge-keep-type { font-size: 10px; background: #e2e8f0; padding: 2px 6px; border-radius: 4px; font-weight: 700; }
.btn-delete-keep { background: none; border: none; font-size: 11px; color: #ef4444; cursor: pointer; }

/* Themes & Settings */
.theme-choice-buttons { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-top: 8px; }
.btn-theme-swatch { padding: 10px; border-radius: 8px; border: 1px solid #cbd5e1; font-size: 12px; font-weight: 700; cursor: pointer; text-align: left; background: white; }
.btn-theme-swatch.active { border-color: #06C755; background: #e8f8ee; }
.setting-toggle-row { display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid #f1f5f9; font-size: 13px; }
.quick-amounts { display: flex; gap: 6px; }
.quick-amounts button { flex: 1; padding: 6px; font-size: 11px; border: 1px solid #cbd5e1; background: white; border-radius: 6px; cursor: pointer; }
.charge-options-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.btn-charge-option { padding: 16px; font-size: 14px; font-weight: 700; border: 1px solid #06C755; color: #06C755; background: white; border-radius: 10px; cursor: pointer; }
.btn-charge-option:hover { background: #e8f8ee; }
.select-friends-scroll { max-height: 180px; overflow-y: auto; border: 1px solid #cbd5e1; border-radius: 8px; padding: 8px; display: flex; flex-direction: column; gap: 8px; }
.friend-check-item { display: flex; align-items: center; gap: 8px; font-size: 13px; cursor: pointer; }
.flex-row { display: flex; gap: 8px; }
</style>
