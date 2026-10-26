<script setup lang="ts">
import { ref } from 'vue'
import { request, type LoginResult } from '../../shared/api'

type Org = { id: string; name: string; state: string; my_role: string }
type Account = { id: string; name: string; status: string }
type Ticket = { id: string; contact_id: string; status: string; priority: string; internal_note: string }

const token = ref(''), email = ref(''), password = ref(''), error = ref(''), note = ref('')
const orgName = ref(''), country = ref('KR'), selectedOrg = ref('')
const accountName = ref(''), selectedAccount = ref('')
const orgs = ref<Org[]>([]), accounts = ref<Account[]>([])
const tickets = ref<Ticket[]>([])

const newTicketContactID = ref(''), newTicketNote = ref(''), newTicketPrio = ref('normal')
const newCampaignName = ref('')
const newCouponCode = ref(''), newCouponRedemptions = ref(50)
const newRichMenuTitle = ref('')

async function run(fn: () => Promise<void>) {
  error.value = ''; note.value = ''
  try { await fn() } catch (e) { error.value = (e as Error).message }
}

async function login() {
  await run(async () => {
    const x = await request<LoginResult>('/api/v1/auth/login', null, 'POST', { email: email.value, password: password.value })
    token.value = x.access_token
    password.value = ''
    await loadOrgs()
  })
}

async function loadOrgs() {
  const x = await request<{ organizations: Org[] }>('/api/v1/partner/organizations', token.value)
  orgs.value = x.organizations
}

async function createOrg() {
  await run(async () => {
    await request('/api/v1/partner/organizations', token.value, 'POST', { name: orgName.value, legal_name: orgName.value, country: country.value })
    orgName.value = ''
    await loadOrgs()
    note.value = '기업 조직 생성 완료'
  })
}

async function selectOrg(id: string) {
  await run(async () => {
    selectedOrg.value = id
    selectedAccount.value = ''
    const x = await request<{ accounts: Account[] }>(`/api/v1/partner/organizations/${id}/accounts`, token.value)
    accounts.value = x.accounts
  })
}

async function createAccount() {
  await run(async () => {
    await request(`/api/v1/partner/organizations/${selectedOrg.value}/accounts`, token.value, 'POST', { name: accountName.value, description: '' })
    accountName.value = ''
    await selectOrg(selectedOrg.value)
    note.value = '공식계정이 생성되었습니다.'
  })
}

async function selectAccount(id: string) {
  selectedAccount.value = id
  await loadTickets()
}

async function loadTickets() {
  if (!selectedAccount.value) return
  const x = await request<{ tickets: Ticket[] }>(`/api/v1/partner/accounts/${selectedAccount.value}/tickets`, token.value)
  tickets.value = x.tickets
}

async function createTicket() {
  await run(async () => {
    await request(`/api/v1/partner/accounts/${selectedAccount.value}/tickets`, token.value, 'POST', {
      contact_id: newTicketContactID.value.trim(),
      priority: newTicketPrio.value,
      internal_note: newTicketNote.value,
    })
    newTicketContactID.value = ''; newTicketNote.value = ''
    await loadTickets()
    note.value = '고객 상담 티켓이 접수되었습니다.'
  })
}

async function createCampaign() {
  await run(async () => {
    await request(`/api/v1/partner/accounts/${selectedAccount.value}/campaigns`, token.value, 'POST', {
      name: newCampaignName.value.trim(),
      message_template: { text: '공식계정 프로모션 알림' },
    })
    newCampaignName.value = ''
    note.value = '브로드캐스트 메시지 캠페인이 등록되었습니다.'
  })
}

async function createCoupon() {
  await run(async () => {
    await request(`/api/v1/partner/accounts/${selectedAccount.value}/coupons`, token.value, 'POST', {
      code: newCouponCode.value.trim(),
      max_redemptions: newCouponRedemptions.value,
    })
    newCouponCode.value = ''
    note.value = '할인 쿠폰이 발급되었습니다.'
  })
}

async function createRichMenu() {
  await run(async () => {
    await request(`/api/v1/partner/accounts/${selectedAccount.value}/rich-menus`, token.value, 'POST', {
      title: newRichMenuTitle.value.trim(),
      definition: { layout: 'grid', buttons: [{ type: 'uri', label: '공식몰' }] },
      enabled: true,
    })
    newRichMenuTitle.value = ''
    note.value = 'Rich Menu가 활성화되었습니다.'
  })
}
</script>

<template>
  <div class="wrap">
    <nav style="display:flex;gap:12px;margin-bottom:18px;padding:10px 14px;background:#fff;border-radius:10px;font-size:13px;align-items:center;border:1px solid #e0e8ee">
      <a href="../" style="text-decoration:none;color:#06754e;font-weight:700">💬 메신저 홈</a>
      <span style="color:#ccc">|</span>
      <a href="../admin/" style="text-decoration:none;color:#334155;font-weight:600">👑 관리자</a>
      <a href="../partner/" style="text-decoration:none;color:#06754e;font-weight:700">🏢 파트너</a>
      <a href="../developer/" style="text-decoration:none;color:#334155;font-weight:600">💻 개발자</a>
      <a href="../creator/" style="text-decoration:none;color:#334155;font-weight:600">🎨 크리에이터</a>
      <a href="../ads/" style="text-decoration:none;color:#334155;font-weight:600">📢 광고주</a>
      <button v-if="token" @click="token = ''" style="margin-left:auto;padding:4px 8px;font-size:12px">로그아웃</button>
    </nav>

    <header>
      <h1>비즈니스 파트너 센터</h1>
      <p>공식계정 · CRM · 고객 상담 · 브로드캐스트 캠페인 · 쿠폰</p>
    </header>

    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="note" class="note">{{ note }}</p>

    <form class="panel" v-if="!token" @submit.prevent="login">
      <h2>파트너 로그인</h2>
      <div style="margin:8px 0;padding:8px 12px;background:#eef5fc;border-radius:8px;font-size:12px;color:#2c5282">
        <strong>💡 기본 계정:</strong> <code>admin@line.local</code> / <code>LineAdmin#2026!</code> 또는 <code>user@line.local</code> / <code>LineUser#2026!</code>
      </div>
      <input v-model="email" type="email" placeholder="이메일" required />
      <input v-model="password" type="password" placeholder="비밀번호" required />
      <button>로그인</button>
    </form>

    <main v-else class="grid">
      <section class="panel">
        <h2>내 기업 조직</h2>
        <form @submit.prevent="createOrg">
          <input v-model="orgName" placeholder="기업명" minlength="2" required />
          <input v-model="country" maxlength="8" placeholder="국가 코드" />
          <button>조직 생성</button>
        </form>
        <div v-for="o in orgs" :key="o.id">
          <button class="item" :class="{ selected: selectedOrg === o.id }" @click="selectOrg(o.id)">
            <strong>{{ o.name }}</strong>
            <small>({{ o.state }} · {{ o.my_role }})</small>
          </button>
        </div>
      </section>

      <section class="panel">
        <template v-if="selectedOrg">
          <h2>공식계정 관리</h2>
          <form @submit.prevent="createAccount">
            <input v-model="accountName" placeholder="공식계정 이름" minlength="2" required />
            <button>공식계정 생성</button>
          </form>
          <div v-for="a in accounts" :key="a.id">
            <button class="item" :class="{ selected: selectedAccount === a.id }" @click="selectAccount(a.id)">
              <strong>{{ a.name }}</strong>
              <small>상태: {{ a.status }}</small>
            </button>
          </div>

          <div v-if="selectedAccount" class="account-tools">
            <h3>상담 티켓 (Shared Inbox)</h3>
            <form @submit.prevent="createTicket" class="mini-form">
              <input v-model="newTicketContactID" placeholder="고객 Contact UUID" required />
              <input v-model="newTicketNote" placeholder="상담 문의 내용" required />
              <select v-model="newTicketPrio">
                <option value="normal">보통</option>
                <option value="urgent">긴급</option>
              </select>
              <button>티켓 접수</button>
            </form>
            <div v-for="t in tickets" :key="t.id" class="ticket-card">
              <span>[{{ t.priority }}] {{ t.internal_note }}</span>
              <small>{{ t.status }}</small>
            </div>

            <h3>캠페인 대량 발송</h3>
            <form @submit.prevent="createCampaign" class="mini-form">
              <input v-model="newCampaignName" placeholder="캠페인 명칭" required />
              <button>캠페인 예약</button>
            </form>

            <h3>프로모션 쿠폰 및 Rich Menu</h3>
            <form @submit.prevent="createCoupon" class="mini-form">
              <input v-model="newCouponCode" placeholder="쿠폰 코드 (예: LINE2026)" required />
              <input type="number" v-model="newCouponRedemptions" min="1" placeholder="수량" />
              <button>쿠폰 생성</button>
            </form>
            <form @submit.prevent="createRichMenu" class="mini-form">
              <input v-model="newRichMenuTitle" placeholder="Rich Menu 타이틀" required />
              <button>Rich Menu 등록</button>
            </form>
          </div>
        </template>
        <p v-else class="muted">왼쪽에서 기업 조직을 선택하세요.</p>
      </section>
    </main>
  </div>
</template>

<style>
body{background:#f3f6fa;margin:0;font:15px system-ui;color:#1d2d45}.wrap{max-width:1150px;margin:auto;padding:25px}.panel{background:white;border-radius:12px;padding:22px;border:1px solid #dce4ed;margin:14px 0}.grid{display:grid;grid-template-columns:360px minmax(0,1fr);gap:20px}input,button,select{padding:10px;border:1px solid #cad5e2;border-radius:9px;margin:4px}button{background:#078452;color:white;cursor:pointer}.item{display:block;margin:8px 0;padding:10px;width:100%;text-align:left;background:#edf3f9;color:#203047;border:1px solid transparent;border-radius:8px}.item.selected{border-color:#078452;background:#dcf2e8}small{color:#6a798b}.account-tools{margin-top:20px;border-top:1px solid #eef1f5;padding-top:15px}.mini-form{display:flex;gap:6px;margin:8px 0}.ticket-card{display:flex;justify-content:space-between;background:#f8fafc;padding:8px 12px;border-radius:6px;margin:6px 0;font-size:13px}.error{color:#b52222;background:#fee;padding:10px;border-radius:8px}.note{color:#087743;background:#eef8f3;padding:10px;border-radius:8px}.muted{color:#6c757d}@media(max-width:760px){.grid{grid-template-columns:1fr}}
</style>
