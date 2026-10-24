<script setup lang="ts">
import { ref } from 'vue'
import { request, type LoginResult } from '../../shared/api'

const token = ref(''), email = ref(''), password = ref(''), name = ref(''), org = ref(''), selectedAdv = ref('')
const campaign = ref(''), objective = ref('conversions'), budget = ref(50000), error = ref(''), note = ref('')
const selectedCamp = ref(''), adGroupName = ref(''), bidAmount = ref(150)

const orgs = ref<Array<{ id: string; name: string }>>([])
const advertisers = ref<Array<{ id: string; name: string; status: string }>>([])
const campaigns = ref<Array<{ id: string; name: string; status: string; daily_budget_minor: number }>>([])

async function run(fn: () => Promise<void>) {
  error.value = ''; note.value = ''
  try { await fn() } catch (e) { error.value = (e as Error).message }
}

async function login() {
  await run(async () => {
    token.value = (await request<LoginResult>('/api/v1/auth/login', null, 'POST', { email: email.value, password: password.value })).access_token
    password.value = ''
    orgs.value = (await request<{ organizations: typeof orgs.value }>('/api/v1/partner/organizations', token.value)).organizations
    await refresh()
  })
}

async function refresh() {
  const [adv, camp] = await Promise.all([
    request<{ advertisers: typeof advertisers.value }>('/api/v1/ads/advertisers', token.value),
    request<{ campaigns: typeof campaigns.value }>('/api/v1/ads/campaigns', token.value),
  ])
  advertisers.value = adv.advertisers
  campaigns.value = camp.campaigns
}

async function addAdvertiser() {
  await run(async () => {
    await request('/api/v1/ads/advertisers', token.value, 'POST', { organization_id: org.value, name: name.value })
    name.value = ''
    await refresh()
    note.value = '광고주 계정이 생성되었습니다.'
  })
}

async function addCampaign() {
  await run(async () => {
    await request('/api/v1/ads/campaigns', token.value, 'POST', {
      advertiser_id: selectedAdv.value,
      name: campaign.value,
      objective: objective.value,
      daily_budget_minor: budget.value,
      currency: 'KRW',
    })
    campaign.value = ''
    await refresh()
    note.value = '광고 캠페인이 생성되었습니다.'
  })
}

async function addAdGroup() {
  if (!selectedCamp.value || !adGroupName.value.trim()) return
  await run(async () => {
    await request(`/api/v1/ads/campaigns/${selectedCamp.value}/groups`, token.value, 'POST', {
      name: adGroupName.value.trim(),
      targeting: { age: '20-40', geo: 'KR' },
      bid_minor: bidAmount.value,
    })
    adGroupName.value = ''
    note.value = '타겟팅 광고 그룹이 추가되었습니다.'
  })
}
</script>

<template>
  <main>
    <h1>Advertiser Center</h1>
    <p>광고주 계정 · 타겟팅 캠페인 · 광고 그룹 및 입찰 예산 관리</p>
    <p role="alert" class="error" v-if="error">{{ error }}</p>
    <p class="note" v-if="note">{{ note }}</p>

    <form v-if="!token" @submit.prevent="login">
      <input v-model="email" type="email" placeholder="이메일" required />
      <input v-model="password" type="password" placeholder="비밀번호" required />
      <button>로그인</button>
    </form>

    <template v-else>
      <section>
        <h2>광고주 계정 생성</h2>
        <form @submit.prevent="addAdvertiser">
          <select v-model="org" required>
            <option value="" disabled>기업 조직 선택</option>
            <option v-for="o in orgs" :key="o.id" :value="o.id">{{ o.name }}</option>
          </select>
          <input v-model="name" placeholder="광고주 이름" required minlength="2" />
          <button>광고주 생성</button>
        </form>
      </section>

      <section>
        <h2>캠페인 생성 및 관리</h2>
        <form @submit.prevent="addCampaign">
          <select v-model="selectedAdv" required>
            <option value="" disabled>광고주 선택</option>
            <option v-for="a in advertisers" :key="a.id" :value="a.id">{{ a.name }} ({{ a.status }})</option>
          </select>
          <input v-model="campaign" placeholder="캠페인 명칭" minlength="2" required />
          <select v-model="objective">
            <option value="conversions">전환 (Conversions)</option>
            <option value="awareness">인지도 (Awareness)</option>
            <option value="traffic">트래픽 (Traffic)</option>
          </select>
          <input v-model.number="budget" type="number" min="0" placeholder="일일 예산 (KRW)" />
          <button>캠페인 생성</button>
        </form>

        <div style="margin-top:16px">
          <h3>내 캠페인 목록 (광고 그룹 설정)</h3>
          <div v-for="c in campaigns" :key="c.id" class="camp-card" :class="{ active: selectedCamp === c.id }" @click="selectedCamp = c.id">
            <strong>{{ c.name }}</strong> · {{ c.status }} · 일일 {{ c.daily_budget_minor.toLocaleString() }} KRW
          </div>

          <div v-if="selectedCamp" class="sub-form">
            <h4>선택한 캠페인에 타겟팅 광고 그룹 추가</h4>
            <form @submit.prevent="addAdGroup">
              <input v-model="adGroupName" placeholder="광고 그룹 명칭 (예: 2040 수도권 타겟)" required />
              <input type="number" v-model="bidAmount" min="10" placeholder="입찰가 (KRW)" />
              <button>광고 그룹 생성</button>
            </form>
          </div>
        </div>
      </section>
    </template>
  </main>
</template>

<style>
body{font:15px system-ui;background:#f4f6fa;color:#172536;margin:0}main{max-width:900px;margin:35px auto;background:white;padding:30px;border-radius:12px}section{border:1px solid #ddd;padding:20px;border-radius:10px;margin:16px 0}input,button,select{padding:10px;border:1px solid #ccc;border-radius:8px;margin:4px}button{background:#145bc0;color:white;cursor:pointer}.camp-card{padding:10px;margin:8px 0;background:#f8fafc;border:1px solid #e2e8f0;border-radius:8px;cursor:pointer}.camp-card.active{border-color:#145bc0;background:#ebf2fc}.sub-form{margin-top:12px;padding:12px;background:#f8fafc;border-radius:8px}.error{color:#a22;background:#ffebeb;padding:10px;border-radius:8px}.note{color:#0b6623;background:#e9f6ec;padding:10px;border-radius:8px}
</style>
