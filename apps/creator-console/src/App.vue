<script setup lang="ts">
import { ref } from 'vue'
import { request, type LoginResult } from '../../shared/api'

const token = ref(''), email = ref(''), password = ref(''), profile = ref(''), title = ref(''), price = ref(2500), error = ref(''), message = ref('')
const selectedPackID = ref(''), stickerObjectKey = ref(''), stickerOrdinal = ref(1)
const packs = ref<Array<{ id: string; title: string; status: string; price_minor: number; currency: string }>>([])
const settlements = ref<Array<{ id: string; period_start: string; period_end: string; amount_minor: number; currency: string; state: string }>>([])

async function run(fn: () => Promise<void>) {
  error.value = ''; message.value = ''
  try { await fn() } catch (e) { error.value = (e as Error).message }
}

async function login() {
  await run(async () => {
    const x = await request<LoginResult>('/api/v1/auth/login', null, 'POST', { email: email.value, password: password.value })
    token.value = x.access_token
    password.value = ''
    await refresh()
  })
}

async function refresh() {
  const [p, s] = await Promise.all([
    request<{ packs: typeof packs.value }>('/api/v1/creator/packs', token.value),
    request<{ settlements: typeof settlements.value }>('/api/v1/creator/settlements', token.value),
  ])
  packs.value = p.packs
  settlements.value = s.settlements
}

async function saveProfile() {
  await run(async () => {
    await request('/api/v1/creator/profile', token.value, 'POST', { display_name: profile.value })
    message.value = '크리에이터 프로필 생성 완료'
  })
}

async function createPack() {
  await run(async () => {
    await request('/api/v1/creator/packs', token.value, 'POST', { title: title.value, price_minor: price.value, currency: 'KRW' })
    title.value = ''
    await refresh()
    message.value = '스티커 팩 초안 생성 완료'
  })
}

async function addStickerItem() {
  if (!selectedPackID.value || !stickerObjectKey.value.trim()) return
  await run(async () => {
    await request(`/api/v1/creator/packs/${selectedPackID.value}/stickers`, token.value, 'POST', {
      object_key: stickerObjectKey.value.trim(),
      kind: 'static',
      ordinal: stickerOrdinal.value,
    })
    stickerObjectKey.value = ''
    stickerOrdinal.value += 1
    message.value = '스티커 아이템이 추가되었습니다.'
  })
}
</script>

<template>
  <main>
    <nav style="display:flex;gap:12px;margin-bottom:18px;padding-bottom:12px;border-bottom:1px solid #e2e8f0;font-size:13px;align-items:center">
      <a href="../" style="text-decoration:none;color:#06754e;font-weight:700">💬 메신저 홈</a>
      <span style="color:#ccc">|</span>
      <a href="../admin/" style="text-decoration:none;color:#334155;font-weight:600">👑 관리자</a>
      <a href="../partner/" style="text-decoration:none;color:#334155;font-weight:600">🏢 파트너</a>
      <a href="../developer/" style="text-decoration:none;color:#334155;font-weight:600">💻 개발자</a>
      <a href="../creator/" style="text-decoration:none;color:#096b52;font-weight:700">🎨 크리에이터</a>
      <a href="../ads/" style="text-decoration:none;color:#334155;font-weight:600">📢 광고주</a>
      <button v-if="token" @click="token = ''" style="margin-left:auto;padding:4px 8px;font-size:12px">로그아웃</button>
    </nav>

    <h1>Creator Market Center</h1>
    <p>크리에이터 프로필 · 스티커/이모티콘 팩 등록 · 개별 스티커 에셋 업로드 · 판매 정산 원장</p>
    <p role="alert" class="error" v-if="error">{{ error }}</p>
    <p class="note" v-if="message">{{ message }}</p>

    <form v-if="!token" @submit.prevent="login">
      <div style="margin:8px 0;padding:8px 12px;background:#eef5fc;border-radius:8px;font-size:12px;color:#2c5282">
        <strong>💡 기본 계정:</strong> <code>admin@line.local</code> / <code>LineAdmin#2026!</code> 또는 <code>user@line.local</code> / <code>LineUser#2026!</code>
      </div>
      <input type="email" v-model="email" placeholder="이메일" required />
      <input v-model="password" type="password" placeholder="비밀번호" required />
      <button>로그인</button>
    </form>

    <template v-else>
      <section>
        <h2>크리에이터 프로필</h2>
        <form @submit.prevent="saveProfile">
          <input v-model="profile" placeholder="크리에이터 표시 이름" minlength="2" required />
          <button>등록 / 수정</button>
        </form>
      </section>

      <section>
        <h2>새 스티커 팩 만들기</h2>
        <form @submit.prevent="createPack">
          <input v-model="title" placeholder="팩 제목 (예: 깜찍이 곰돌이)" minlength="2" required />
          <input v-model.number="price" type="number" min="0" placeholder="가격 (KRW)" />
          <button>초안 생성</button>
        </form>
      </section>

      <section>
        <h2>내 스티커 팩 및 스티커 에셋 등록</h2>
        <div v-for="p in packs" :key="p.id" class="pack-card" :class="{ active: selectedPackID === p.id }" @click="selectedPackID = p.id">
          <strong>{{ p.title }}</strong> · {{ p.price_minor }} {{ p.currency }} ({{ p.status }})
        </div>

        <div v-if="selectedPackID" class="sub-form">
          <h3>선택한 팩에 스티커 이미지 에셋 추가</h3>
          <form @submit.prevent="addStickerItem">
            <input v-model="stickerObjectKey" placeholder="스토리지 키 (예: stickers/pack-01/01.png)" required />
            <input type="number" v-model="stickerOrdinal" min="1" placeholder="순서 (1, 2, ...)" />
            <button>스티커 아이템 추가</button>
          </form>
        </div>
      </section>

      <section>
        <h2>수익 정산 내역 (Settlements Ledger)</h2>
        <p v-if="!settlements.length">현재 정산 내역이 없습니다.</p>
        <div v-for="s in settlements" :key="s.id" class="item">
          <span>정산 기간: {{ s.period_start }} ~ {{ s.period_end }}</span>
          <strong>{{ s.amount_minor.toLocaleString() }} {{ s.currency }} ({{ s.state }})</strong>
        </div>
      </section>
    </template>
  </main>
</template>

<style>
body{font:15px system-ui;background:#f4f6fa;color:#172536;margin:0}main{max-width:900px;margin:35px auto;background:white;padding:30px;border-radius:12px}section{border:1px solid #ddd;padding:20px;border-radius:10px;margin:16px 0}input,button{padding:10px;border:1px solid #ccc;border-radius:8px;margin:4px}button{background:#096b52;color:white;cursor:pointer}.pack-card{padding:10px;margin:8px 0;background:#f8fafc;border:1px solid #e2e8f0;border-radius:8px;cursor:pointer}.pack-card.active{border-color:#096b52;background:#e9f6f2}.sub-form{margin-top:14px;padding:12px;background:#f8fafc;border-radius:8px}.item{display:flex;justify-content:space-between;padding:8px 0;border-bottom:1px solid #eee}.error{color:#a22;background:#ffebeb;padding:10px;border-radius:8px}.note{color:#176b3e;background:#e8f7ee;padding:10px;border-radius:8px}
</style>
