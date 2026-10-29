<template>
  <div class="home-view">
    <div class="line-header">
      <div class="line-header-title">홈</div>
      <div class="line-header-actions">
        <LineIcons name="search" class="btn-icon" />
        <LineIcons name="settings" class="btn-icon" />
      </div>
    </div>

    <div class="line-content home-content">
      <div class="profile-section">
        <div class="profile-header">
          <div class="avatar avatar-lg">
            {{ currentUser?.display_name?.slice(0, 1) || 'L' }}
          </div>
          <div class="profile-info">
            <h1 class="profile-name">{{ currentUser?.display_name || '내 프로필' }}</h1>
            <p class="profile-status">{{ currentUser?.status_message || '상태 메시지가 없습니다' }}</p>
          </div>
          <LineIcons name="more" class="btn-icon" />
        </div>
        <div class="profile-quick-actions">
          <button class="quick-action">
            <LineIcons name="chat" class="btn-icon" />
            <span>채팅</span>
          </button>
          <button class="quick-action">
            <LineIcons name="voom" class="btn-icon" />
            <span>VOOM</span>
          </button>
          <button class="quick-action">
            <LineIcons name="openchat" class="btn-icon" />
            <span>오픈채팅</span>
          </button>
          <button class="quick-action">
            <LineIcons name="wallet" class="btn-icon" />
            <span>월렛</span>
          </button>
        </div>
      </div>

      <div class="divider-text"><span>친구</span></div>

      <div class="favorites-section">
        <div class="section-header">
          <h2>즐겨찾기</h2>
        </div>
        <div class="friends-list">
          <div v-for="f in favoriteFriends" :key="f.id" class="list-item friend-item" @click="openChat(f.id)">
            <div class="avatar">
              {{ f.display_name?.slice(0, 1) || 'F' }}
            </div>
            <div class="list-item-content">
              <div class="list-item-title">{{ f.display_name }}</div>
              <div class="list-item-subtitle">
                <span class="badge" v-if="f.is_favorite">★</span>
                {{ f.status_message || '' }}
              </div>
            </div>
            <LineIcons name="more" class="btn-icon" />
          </div>
          <div v-if="favoriteFriends.length === 0" class="empty-state">
            즐겨찾기한 친구가 없습니다.
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import LineIcons from './LineIcons.vue'

const props = defineProps({
  currentUser: { type: Object, default: null },
  favoriteFriends: { type: Array, default: () => [] },
  birthdayFriends: { type: Array, default: () => [] },
  officialAccounts: { type: Array, default: () => [] }
})

const emit = defineEmits(['open-chat', 'follow-account'])

const openChat = (id) => emit('open-chat', id)
const followAccount = (id) => emit('follow-account', id)
</script>

<style scoped>
.home-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.home-content {
  flex: 1;
  overflow-y: auto;
}

.profile-section {
  padding: var(--line-space-4);
  background-color: var(--line-bg-panel);
}

.profile-header {
  display: flex;
  align-items: center;
  gap: var(--line-space-3);
  margin-bottom: var(--line-space-4);
}

.profile-name {
  font-size: var(--line-font-size-xl);
  font-weight: var(--line-font-weight-bold);
  margin: 0 0 var(--line-space-1) 0;
}

.profile-status {
  font-size: var(--line-font-size-sm);
  color: var(--line-text-secondary);
  margin: 0;
}

.profile-quick-actions {
  display: flex;
  gap: var(--line-space-2);
  overflow-x: auto;
  padding-bottom: var(--line-space-2);
  margin-bottom: var(--line-space-4);
  scrollbar-width: none;
}

.profile-quick-actions::-webkit-scrollbar {
  display: none;
}

.quick-action {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--line-space-1);
  padding: var(--line-space-2);
  min-width: 60px;
  color: var(--line-text-secondary);
  font-size: var(--line-font-size-xs);
  font-weight: var(--line-font-weight-medium);
  border-radius: var(--line-radius-md);
  background: transparent;
}

.quick-action .btn-icon {
  width: 40px;
  height: 40px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--line-space-4) var(--line-space-3) var(--line-space-4);
}

.section-header h2 {
  font-size: var(--line-font-size-lg);
  font-weight: var(--line-font-weight-bold);
  margin: 0;
}

.friends-list {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.empty-state {
  padding: var(--line-space-6);
  text-align: center;
  color: var(--line-text-muted);
  font-size: var(--line-font-size-sm);
}

.btn-sm {
  height: 28px;
  padding: 0 var(--line-space-3);
  font-size: var(--line-font-size-xs);
}
</style>