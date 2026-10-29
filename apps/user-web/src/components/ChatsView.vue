<template>
  <div class="chats-view">
    <div class="line-header">
      <div class="line-header-title">채팅</div>
      <div class="line-header-actions">
        <LineIcons name="search" class="btn-icon" />
        <LineIcons name="more" class="btn-icon" />
      </div>
    </div>

    <div class="line-content chats-content">
      <div class="chat-filter-bar">
        <button
          v-for="filter in chatFilters"
          :key="filter.id"
          :class="['filter-btn', { active: chatFilter === filter.id }]"
          @click="chatFilter = filter.id"
        >
          {{ filter.label }}
        </button>
      </div>

      <div class="search-box">
        <input
          v-model="searchKeyword"
          type="text"
          placeholder="대화방 또는 메시지 검색"
          class="input"
        />
      </div>

      <div class="chats-list">
        <div
          v-for="c in filteredChats"
          :key="c.id"
          class="list-item chat-row"
          :class="{ selected: selectedChatId === c.id }"
          @click="selectChat(c.id)"
        >
          <div class="avatar chat-avatar" :class="{ 'group-avatar': c.kind === 'group' }">
            {{ c.title ? c.title.slice(0, 1) : 'L' }}
          </div>
          <div class="list-item-content">
            <div class="chat-title-line">
              <strong class="chat-title">{{ c.title || '대화방' }}</strong>
              <span v-if="c.kind === 'group'" class="member-count">{{ c.member_count || 2 }}명</span>
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
        <div v-if="filteredChats.length === 0" class="empty-state">
          대화방이 없습니다.
        </div>
      </div>
    </div>

    <button class="fab" @click="newChat">
      <LineIcons name="chat" />
    </button>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import LineIcons from './LineIcons.vue'

const props = {
  chats: { type: Array, default: () => [] },
  selectedChatId: { type: String, default: '' }
}

const emit = defineEmits(['select-chat', 'new-chat'])

const chatFilter = ref('all')
const searchKeyword = ref('')

const chatFilters = [
  { id: 'all', label: '전체' },
  { id: 'direct', label: '1:1' },
  { id: 'group', label: '그룹' },
  { id: 'unread', label: '안읽음' }
]

const filteredChats = computed(() => {
  let result = props.chats
  if (chatFilter.value === 'direct') {
    result = result.filter(c => c.kind === 'direct')
  } else if (chatFilter.value === 'group') {
    result = result.filter(c => c.kind === 'group')
  } else if (chatFilter.value === 'unread') {
    result = result.filter(c => (c.unread_count || 0) > 0)
  }
  if (searchKeyword.value) {
    const kw = searchKeyword.value.toLowerCase()
    result = result.filter(c =>
      (c.title || '').toLowerCase().includes(kw) ||
      (c.last_message?.body || '').toLowerCase().includes(kw)
    )
  }
  return result
})

const formatTime = (iso) => {
  if (!iso) return ''
  const d = new Date(iso)
  const now = new Date()
  const diff = now - d
  if (diff < 86400000) {
    return d.toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString('ko-KR', { month: '2-digit', day: '2-digit' })
}

const selectChat = (id) => emit('select-chat', id)
const newChat = () => emit('new-chat')
</script>

<style scoped>
.chats-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.chats-content {
  flex: 1;
  overflow-y: auto;
}

.chat-filter-bar {
  display: flex;
  gap: var(--line-space-1);
  padding: var(--line-space-2) var(--line-space-3);
  background-color: var(--line-bg-panel);
  border-bottom: 1px solid var(--line-border);
  overflow-x: auto;
  scrollbar-width: none;
}

.chat-filter-bar::-webkit-scrollbar {
  display: none;
}

.filter-btn {
  height: 32px;
  padding: 0 var(--line-space-3);
  border-radius: var(--line-radius-full);
  font-size: var(--line-font-size-sm);
  font-weight: var(--line-font-weight-medium);
  color: var(--line-text-secondary);
  background: transparent;
  white-space: nowrap;
  transition: all var(--line-transition-fast);
}

.filter-btn.active {
  color: var(--line-green);
  background-color: var(--line-green-lighter);
}

.search-box {
  padding: var(--line-space-2) var(--line-space-3);
  background-color: var(--line-bg-panel);
  border-bottom: 1px solid var(--line-border);
}

.chats-list {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.chat-avatar.group-avatar {
  background-color: var(--line-bg-tertiary);
  color: var(--line-text-muted);
}

.badge-pin {
  color: var(--line-warning);
  font-size: var(--line-font-size-xs);
  margin-left: var(--line-space-1);
}

.unread-badge {
  background-color: var(--line-green);
  color: white;
  font-size: var(--line-font-size-xs);
  font-weight: var(--line-font-weight-bold);
  padding: 2px 6px;
  border-radius: var(--line-radius-full);
  margin-left: var(--line-space-1);
}

.empty-state {
  padding: var(--line-space-6);
  text-align: center;
  color: var(--line-text-muted);
  font-size: var(--line-font-size-sm);
}

.fab {
  position: fixed;
  bottom: 100px;
  right: 24px;
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background: var(--line-green);
  color: white;
  font-size: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 12px rgba(0, 195, 0, 0.4);
  z-index: 50;
}

.fab .line-icon {
  width: 24px;
  height: 24px;
}
</style>