<template>
  <div class="openchat-view">
    <div class="line-header">
      <div class="line-header-title">오픈채팅</div>
      <div class="line-header-actions">
        <LineIcons name="plus" class="btn-icon" />
        <LineIcons name="search" class="btn-icon" />
      </div>
    </div>

    <div class="line-content openchat-content">
      <!-- 커뮤니티 방 리스트 -->
      <div class="community-rooms">
        <div v-for="room in communityRooms" :key="room.id" class="room-item" @click="enterRoom(room.id)">
          <div class="room-avatar">
            <div class="badge" :style="{ background: room.color }">
              {{ room.name.slice(0, 1) }}
            </div>
          </div>
          <div class="room-info">
            <div class="room-name">{{ room.name }}</div>
            <div class="room-description">{{ room.description }}</div>
          </div>
          <div class="room-stats">
            <span>{{ room.member_count }}명</span>
            <span>{{ room.post_count }}글</span>
          </div>
        </div>
        <div v-if="communityRooms.length === 0" class="empty-state">
          검색된 오픈채팅방이 없습니다.
        </div>
      </div>

      <!-- 검색 및 생성 -->
      <div class="search-and-create">
        <div class="input-group">
          <input
            v-model="searchKeyword"
            type="text"
            placeholder="오픈채팅방 검색 또는 생성"
            class="input"
          />
          <button class="btn-primary" @click="createRoom">방 생성</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import LineIcons from './LineIcons.vue'

const props = {
  communityRooms: { type: Array, default: () => [] },
  searchKeyword: { type: String, default: '' }
}

const emit = defineEmits(['create-room'])

const createRoom = () => emit('create-room')
</script>

<style scoped>
.openchat-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.openchat-content {
  flex: 1;
  overflow-y: auto;
  background-color: var(--line-bg-chat);
}

.community-rooms {
  padding: var(--line-space-3);
}

.room-item {
  display: flex;
  align-items: center;
  padding: var(--line-space-3);
  gap: var(--line-space-3);
  border-radius: var(--line-radius-md);
  cursor: pointer;
  transition: background-color var(--line-transition-fast);
}

.room-item:hover {
  background-color: var(--line-bg-secondary);
}

.room-avatar {
  flex-shrink: 0;
}

.badge {
  width: 28px;
  height: 28px;
  border-radius: var(--line-radius-full);
  font-size: var(--line-font-size-sm);
  font-weight: var(--line-font-weight-bold);
  color: var(--line-text-inverse);
  display: flex;
  align-items: center;
  justify-content: center;
}

.room-info {
  flex: 1;
}

.room-name {
  font-size: var(--line-font-size-base);
  font-weight: var(--line-font-weight-medium);
  color: var(--line-text-primary);
}

.room-description {
  font-size: var(--line-font-size-sm);
  color: var(--line-text-secondary);
  margin-top: 2px;
}

.room-stats {
  margin-top: var(--line-space-1);
  display: flex;
  gap: var(--line-space-4);
  font-size: var(--line-font-size-xs);
  color: var(--line-text-muted);
}

.search-and-create {
  padding: var(--line-space-3);
  background-color: var(--line-bg-panel);
  border-top: 1px solid var(--line-border);
}

.input-group {
  display: flex;
  gap: var(--line-space-2);
}

.search-and-create .input {
  flex: 1;
}

.btn-primary.wide {
  padding: 0 var(--line-space-4);
  min-width: 80px;
}
</style>