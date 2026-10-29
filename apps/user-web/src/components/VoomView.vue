<template>
  <div class="voom-view">
    <div class="line-header">
      <div class="line-header-title">VOOM</div>
      <div class="line-header-actions">
        <LineIcons name="plus" class="btn-icon" title="새 스토리" @click="showNewStoryModal = true" />
        <LineIcons name="search" class="btn-icon" />
      </div>
    </div>

    <div class="line-content voom-content">
      <!-- 스토리 캐러셀 -->
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

      <!-- 숏폼 비디오 피드 -->
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
              <button class="btn-icon" @click="shareVoomPost(v.id)">
                <LineIcons name="more" />
              </button>
            </div>
          </div>
        </div>
        <div v-if="voomVideos.length === 0" class="empty-state">
          표시할 콘텐츠가 없습니다.
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import LineIcons from './LineIcons.vue'

const props = {
  stories: { type: Array, default: () => [] },
  voomVideos: { type: Array, default: () => [] },
  showNewStoryModal: { type: Boolean, default: false }
}

const emit = defineEmits(['new-story', 'like-voom', 'share-voom'])

const likeVoomPost = (id) => emit('like-voom', id)
const shareVoomPost = (id) => emit('share-voom', id)
</script>

<style scoped>
.voom-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.voom-content {
  flex: 1;
  overflow-y: auto;
  background-color: var(--line-bg-chat);
}

.stories-row {
  display: flex;
  padding: var(--line-space-3);
  background-color: var(--line-bg-panel);
  border-bottom: 1px solid var(--line-border);
}

.story-item {
  flex: 0 0 auto;
  margin-right: var(--line-space-2);
  cursor: pointer;
}

.story-ring {
  width: 64px;
  height: 64px;
  border-radius: var(--line-radius-full);
  border: 3px solid var(--line-border);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--line-font-size-lg);
  font-weight: var(--line-font-weight-bold);
  color: var(--line-text-primary);
  background-color: var(--line-bg-primary);
  margin-bottom: var(--line-space-1);
}

.story-ring.plus {
  border-color: var(--line-green);
  background-color: var(--line-green);
  color: var(--line-text-inverse);
}

.story-ring.new {
  border-color: var(--line-green);
  box-shadow: 0 0 0 4px var(--line-green);
}

.story-ring.viewed {
  border-color: var(--line-text-muted);
}

.voom-list {
  padding: var(--line-space-3);
}

.voom-card {
  margin-bottom: var(--line-space-4);
}

.voom-banner {
  width: 100%;
  height: 180px;
  background-color: var(--line-bg-tertiary);
  position: relative;
}

.voom-emoji {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: var(--line-font-size-2xl);
}

.voom-meta {
  padding: var(--line-space-2) 0;
}

.voom-actions {
  display: flex;
  gap: var(--line-space-2);
  margin-top: var(--line-space-2);
  opacity: 0;
  transition: opacity var(--line-transition-fast);
}

.voom-card:hover .voom-actions {
  opacity: 1;
}

.empty-state {
  padding: var(--line-space-6);
  text-align: center;
  color: var(--line-text-muted);
  font-size: var(--line-font-size-sm);
}
</style>