<template>
  <div class="chat-room-view">
    <div class="line-header">
      <div class="chat-header">
        <button class="btn-icon" @click="$emit('back')">
          <LineIcons name="chat" />
        </button>
        <div class="chat-header-info">
          <div class="avatar chat-avatar" :class="{ 'group-avatar': selectedChat?.kind === 'group' }">
            {{ selectedChat?.title ? selectedChat.title.slice(0, 1) : 'L' }}
          </div>
          <div class="chat-header-text">
            <div class="chat-header-name">{{ selectedChat?.title || '대화방' }}</div>
            <div class="chat-header-status">{{ memberCount }}명 · {{ onlineStatus }}</div>
          </div>
        </div>
        <div class="line-header-actions">
          <LineIcons name="search" class="btn-icon" />
          <LineIcons name="more" class="btn-icon" />
        </div>
      </div>
    </div>

    <div class="line-content chat-content">
      <!-- 메시지 목록 -->
      <div class="message-list" ref="messageList">
        <!-- 날짜 구분선 -->
        <div v-for="dateGroup in dateGroups" :key="dateGroup.date" class="date-separator">
          {{ dateGroup.label }}
        </div>

        <!-- 메시지 -->
        <div
          v-for="msg in messages"
          :key="msg.client_id || msg.id"
          class="message-bubble-container"
          :class="{ mine: msg.sender_id === currentUserId, other: msg.sender_id !== currentUserId }"
        >
          <div class="chat-bubble" :class="{ mine: msg.sender_id === currentUserId, other: msg.sender_id !== currentUserId, unsent: msg.unsent }">
            {{ msg.unsent ? '메시지를 취소했습니다' : msg.body }}
          </div>
          <div class="message-meta">
            <span class="chat-time" :class="{ mine: msg.sender_id === currentUserId, other: msg.sender_id !== currentUserId }">
              {{ formatTime(msg.created_at) }}
            </span>
            <span v-if="msg.sender_id === currentUserId" class="read-status" :class="{ read: msg.read }">
              {{ msg.read ? '읽음' : '' }}
            </span>
          </div>
        </div>

        <!-- 답장 미리보기 -->
        <div v-if="replyToMessage" class="reply-preview">
          <button class="btn-icon" @click="replyToMessage = null">✕</button>
          <div class="reply-content">
            <div class="reply-author">{{ replyToMessage.sender_name }}</div>
            <div class="reply-body">{{ replyToMessage.body }}</div>
          </div>
        </div>
      </div>

      <!-- 하단 작곡자 -->
      <div class="message-composer">
        <button class="composer-btn" @click="$emit('attach')">
          <LineIcons name="more" />
        </button>
        <input
          v-model="draftMessage"
          class="composer-input"
          placeholder="메시지 입력"
          @keydown.enter.prevent="sendMessage"
        />
        <button class="composer-btn active" @click="sendMessage">
          <LineIcons name="send" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import LineIcons from './LineIcons.vue'

const props = {
  selectedChat: { type: Object, default: null },
  messages: { type: Array, default: () => [] },
  currentUserId: { type: String, default: '' }
}

const emit = defineEmits(['back', 'attach', 'send-message'])

const draftMessage = ref('')
const replyToMessage = ref(null)
const messageList = ref(null)

const memberCount = computed(() => props.selectedChat?.member_count || 2)
const onlineStatus = computed(() => '온라인')

const dateGroups = computed(() => {
  const groups = []
  let currentDate = null
  for (const msg of props.messages) {
    const msgDate = new Date(msg.created_at).toDateString()
    if (msgDate !== currentDate) {
      currentDate = msgDate
      groups.push({
        date: currentDate,
        label: formatDate(msg.created_at)
      })
    }
  }
  return groups
})

const formatTime = (iso) => {
  if (!iso) return ''
  const d = new Date(iso)
  return d.toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit' })
}

const formatDate = (iso) => {
  if (!iso) return ''
  const d = new Date(iso)
  const today = new Date()
  if (d.toDateString() === today.toDateString()) return '오늘'
  const yesterday = new Date(today)
  yesterday.setDate(yesterday.getDate() - 1)
  if (d.toDateString() === yesterday.toDateString()) return '어제'
  return d.toLocaleDateString('ko-KR', { month: '2-digit', day: '2-digit' })
}

const sendMessage = () => {
  if (!draftMessage.value.trim()) return
  emit('send-message', draftMessage.value)
  draftMessage.value = ''
}

const scrollToBottom = () => {
  nextTick(() => {
    if (messageList.value) {
      messageList.value.scrollTop = messageList.value.scrollHeight
    }
  })
}

onMounted(() => {
  scrollToBottom()
})

// 메시지 변경 시 하단으로 스크롤
const watch = require('vue').watch
watch(() => props.messages, scrollToBottom)
</script>

<style scoped>
.chat-room-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.chat-header {
  display: flex;
  align-items: center;
  gap: var(--line-space-3);
  padding: 0 var(--line-space-3);
}

.chat-header-info {
  display: flex;
  align-items: center;
  gap: var(--line-space-2);
}

.chat-header-name {
  font-size: var(--line-font-size-lg);
  font-weight: var(--line-font-weight-bold);
  color: var(--line-text-primary);
}

.chat-header-status {
  font-size: var(--line-font-size-sm);
  color: var(--line-text-secondary);
}

.chat-avatar.group-avatar {
  background-color: var(--line-bg-tertiary);
  color: var(--line-text-muted);
}

.chat-content {
  flex: 1;
  overflow-y: auto;
  background-color: var(--line-bg-chat);
  display: flex;
  flex-direction: column;
}

.message-list {
  flex: 1;
  overflow-y: auto;
  padding: var(--line-space-3);
  display: flex;
  flex-direction: column;
  gap: var(--line-space-2);
}

.date-separator {
  display: flex;
  justify-content: center;
  align-items: center;
  margin: var(--line-space-4) 0;
  font-size: var(--line-font-size-xs);
  color: var(--line-text-muted);
  text-align: center;
}

.message-meta {
  display: flex;
  align-items: baseline;
  gap: 4px;
  font-size: var(--line-font-size-xs);
}

.message-meta.mine {
  justify-content: flex-end;
}

.message-meta.other {
  justify-content: flex-start;
}

.read-status {
  color: var(--line-text-muted);
}

.read-status.read {
  color: var(--line-green);
}

.reply-preview {
  display: flex;
  align-items: center;
  gap: var(--line-space-2);
  padding: var(--line-space-3);
  background-color: var(--line-bg-secondary);
  border-bottom: 1px solid var(--line-border);
}

.reply-content {
  flex: 1;
}

.reply-author {
  font-size: var(--line-font-size-xs);
  font-weight: var(--line-font-weight-bold);
  color: var(--line-text-primary);
}

.reply-body {
  font-size: var(--line-font-size-sm);
  color: var(--line-text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.message-composer {
  display: flex;
  align-items: flex-end;
  gap: var(--line-space-2);
  padding: var(--line-space-2);
  background-color: var(--line-bg-panel);
  border-top: 1px solid var(--line-border);
}

.composer-input {
  flex: 1;
  height: 40px;
  padding: 0 var(--line-space-4);
  border: 1px solid var(--line-border);
  border-radius: var(--line-radius-full);
  background-color: var(--line-bg-primary);
  font-size: var(--line-font-size-base);
  color: var(--line-text-primary);
  outline: none;
}

.composer-input:focus {
  border-color: var(--line-green);
}

.composer-btn {
  width: 40px;
  height: 40px;
  border-radius: var(--line-radius-full);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--line-text-secondary);
}

.composer-btn.active {
  color: var(--line-green);
  background-color: var(--line-green-lighter);
}
</style>