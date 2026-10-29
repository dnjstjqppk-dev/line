<template>
  <div class="wallet-view">
    <div class="line-header">
      <div class="line-header-title">월렛</div>
      <div class="line-header-actions">
        <LineIcons name="more" class="btn-icon" />
      </div>
    </div>

    <div class="line-content wallet-content">
      <!-- 잔액 표시 -->
      <div class="wallet-balance">
        <div class="balance-amount">{{ walletBalance }}</div>
        <div class="balance-currency">원</div>
      </div>

      <!-- 충전 버튼 -->
      <div class="charge-btn">
        <button class="btn-primary wide" @click="showChargeModal = true">
          <LineIcons name="send" class="btn-icon" /> 잔액 충전
        </button>
      </div>

      <!-- 최근 거래 내역 -->
      <div class="transaction-history">
        <div class="section-header">
          <h2>최근 거래</h2>
        </div>
        <div v-for="tx in walletTx" :key="tx.id" class="transaction-item">
          <div class="transaction-type">
            <span v-if="tx.type === 'charge'">충전</span>
            <span v-else>송금</span>
          </div>
          <div class="transaction-amount">
            <span v-if="tx.type === 'charge'">{{ tx.amount | formatCurrency }}</span>
            <span v-else>{{ '-' + tx.amount | formatCurrency }}</span>
          </div>
          <div class="transaction-note">{{ tx.note || '' }}</div>
          <div class="transaction-time">{{ formatTime(tx.created_at) }}</div>
        </div>
        <div v-if="walletTx.length === 0" class="empty-state">
          거래 내역이 없습니다.
        </div>
      </div>

      <!-- 서비스 그리드 -->
      <div class="service-grid">
        <div class="section-header">
          <h2>서비스</h2>
        </div>
        <div class="service-grid-items">
          <div
            v-for="service in services"
            :key="service.id"
            class="service-item"
            @click="openService(service.id)"
          >
            <div class="service-icon">
              <LineIcons :name="service.icon" class="service-icon-svg" />
            </div>
            <div class="service-name">{{ service.name }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import LineIcons from './LineIcons.vue'

const props = {
  walletBalance: { type: Number, default: 50000 },
  walletTx: { type: Array, default: () => [] },
  showChargeModal: { type: Boolean, default: false }
}

const emit = defineEmits(['charge', 'open-service'])

const services = [
  { id: 'gift', name: '선물하기', icon: 'more' },
  { id: 'stickershop', name: '스티커샵', icon: 'chat' },
  { id: 'themeshop', name: '테마샵', icon: 'settings' },
  { id: 'keep', name: '보관함', icon: 'pin' },
  { id: 'profile', name: '내 프로필', icon: 'profile' }
]

const formatCurrency = (num) => num.toLocaleString('ko-KR')

const formatTime = (iso) => {
  if (!iso) return ''
  const d = new Date(iso)
  return d.toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit' })
}
</script>

<style scoped>
.wallet-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.wallet-content {
  flex: 1;
  overflow-y: auto;
  background-color: var(--line-bg-chat);
}

.wallet-balance {
  padding: var(--line-space-4) var(--line-space-3);
  text-align: center;
}

.balance-amount {
  font-size: var(--line-font-size-2xl);
  font-weight: var(--line-font-weight-bold);
  color: var(--line-green);
  margin: 0 0 var(--line-space-1) 0;
}

.balance-currency {
  font-size: var(--line-font-size-sm);
  color: var(--line-text-secondary);
}

.charge-btn {
  padding: var(--line-space-3) var(--line-space-4);
  text-align: center;
}

.service-grid {
  padding: var(--line-space-3);
}

.service-grid-items {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: var(--line-space-3);
}

.service-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--line-space-2);
  padding: var(--line-space-3);
  border-radius: var(--line-radius-md);
  cursor: pointer;
  transition: background-color var(--line-transition-fast);
}

.service-item:hover {
  background-color: var(--line-bg-secondary);
}

.service-icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.service-icon-svg {
  width: 24px;
  height: 24px;
  color: var(--line-green);
}

.service-name {
  font-size: var(--line-font-size-sm);
  color: var(--line-text-primary);
  text-align: center;
}

.empty-state {
  padding: var(--line-space-6);
  text-align: center;
  color: var(--line-text-muted);
  font-size: var(--line-font-size-sm);
}
</style>