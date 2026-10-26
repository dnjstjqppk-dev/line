<script setup lang="ts">
import { ref } from 'vue'
import { request, type LoginResult } from '../../shared/api'

const token = ref(''), email = ref(''), password = ref(''), name = ref(''), appName = ref(''), orgID = ref(''), selectedAppID = ref(''), error = ref(''), note = ref(''), oneTimeSecret = ref('')
const webhookURL = ref(''), webhookSecret = ref(''), miniAppOrigin = ref('')
const orgs = ref<Array<{ id: string; name: string; role: string }>>([])
const apps = ref<Array<{ id: string; name: string; client_id: string; status: string }>>([])

async function run(fn: () => Promise<void>) {
  error.value = ''; note.value = ''
  try { await fn() } catch (e) { error.value = (e as Error).message }
}

async function login() {
  await run(async () => {
    const x = await request<LoginResult>('/api/v1/auth/login', null, 'POST', { email: email.value, password: password.value })
    token.value = x.access_token
    await refresh()
  })
}

async function refresh() {
  const x = await request<{ organizations: typeof orgs.value }>('/api/v1/developer/organizations', token.value)
  orgs.value = x.organizations
}

async function add() {
  await run(async () => {
    await request('/api/v1/developer/organizations', token.value, 'POST', { name: name.value })
    name.value = ''
    await refresh()
    note.value = '개발자 조직이 생성되었습니다.'
  })
}

async function selectOrg(id: string) {
  await run(async () => {
    orgID.value = id
    selectedAppID.value = ''
    apps.value = (await request<{ apps: typeof apps.value }>(`/api/v1/developer/organizations/${id}/apps`, token.value)).apps
  })
}

async function createApp() {
  await run(async () => {
    const x = await request<{ id: string; client_secret_once: string }>(`/api/v1/developer/organizations/${orgID.value}/apps`, token.value, 'POST', { name: appName.value })
    oneTimeSecret.value = x.client_secret_once
    appName.value = ''
    await selectOrg(orgID.value)
    note.value = 'API 앱이 생성되었습니다. 시크릿 키를 안전하게 보관하세요.'
  })
}

async function registerWebhook() {
  if (!selectedAppID.value || !webhookURL.value.trim()) return
  await run(async () => {
    const res = await request<{ id: string; signing_secret_once: string }>(`/api/v1/developer/apps/${selectedAppID.value}/webhooks`, token.value, 'POST', {
      endpoint_url: webhookURL.value.trim(),
    })
    webhookSecret.value = res.signing_secret_once
    webhookURL.value = ''
    note.value = 'Webhook 엔드포인트가 등록되었습니다.'
  })
}

async function registerMiniApp() {
  if (!selectedAppID.value || !miniAppOrigin.value.trim()) return
  await run(async () => {
    await request(`/api/v1/developer/apps/${selectedAppID.value}/miniapps`, token.value, 'POST', {
      origin: miniAppOrigin.value.trim(),
      manifest: { name: 'Sample MiniApp', version: '1.0.0' },
    })
    miniAppOrigin.value = ''
    note.value = 'MiniApp 심사 등록이 완료되었습니다.'
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
      <a href="../developer/" style="text-decoration:none;color:#165dd8;font-weight:700">💻 개발자</a>
      <a href="../creator/" style="text-decoration:none;color:#334155;font-weight:600">🎨 크리에이터</a>
      <a href="../ads/" style="text-decoration:none;color:#334155;font-weight:600">📢 광고주</a>
      <button v-if="token" @click="token = ''" style="margin-left:auto;padding:4px 8px;font-size:12px">로그아웃</button>
    </nav>

    <h1>Developer Center</h1>
    <p>개발자 조직 · Messaging API 앱 · Webhook 발송 엔진 · Mini App 플랫폼</p>
    <p class="error" role="alert" v-if="error">{{ error }}</p>
    <p class="note" v-if="note">{{ note }}</p>

    <form @submit.prevent="login" v-if="!token">
      <div style="margin:8px 0;padding:8px 12px;background:#eef5fc;border-radius:8px;font-size:12px;color:#2c5282">
        <strong>💡 기본 계정:</strong> <code>admin@line.local</code> / <code>LineAdmin#2026!</code> 또는 <code>user@line.local</code> / <code>LineUser#2026!</code>
      </div>
      <input v-model="email" type="email" placeholder="이메일" required />
      <input v-model="password" type="password" placeholder="비밀번호" required />
      <button>로그인</button>
    </form>

    <template v-else>
      <section>
        <h2>개발자 조직</h2>
        <form @submit.prevent="add">
          <input v-model="name" placeholder="조직명" minlength="2" required />
          <button>조직 생성</button>
        </form>
        <button v-for="o in orgs" :key="o.id" :class="{ active: orgID === o.id }" @click="selectOrg(o.id)">
          {{ o.name }} ({{ o.role }})
        </button>
      </section>

      <section v-if="orgID">
        <h2>API 앱 관리</h2>
        <form @submit.prevent="createApp">
          <input v-model="appName" placeholder="앱 이름" minlength="2" required />
          <button>앱 생성</button>
        </form>

        <p class="secret" v-if="oneTimeSecret">
          한 번만 표시되는 Client Secret (안전하게 보관): <code>{{ oneTimeSecret }}</code>
          <button @click="oneTimeSecret = ''" style="margin-left:8px">닫기</button>
        </p>

        <div v-for="a in apps" :key="a.id" class="app-card" :class="{ active: selectedAppID === a.id }" @click="selectedAppID = a.id">
          <strong>{{ a.name }}</strong> ({{ a.status }})<br />
          <small>Client ID: <code>{{ a.client_id }}</code></small>
        </div>

        <div v-if="selectedAppID" class="tool-box">
          <h3>Webhook 등록</h3>
          <form @submit.prevent="registerWebhook">
            <input v-model="webhookURL" placeholder="https://mybot.example.com/webhook" required />
            <button>Webhook 등록</button>
          </form>
          <p v-if="webhookSecret" class="secret">
            Webhook 서명 시크릿: <code>{{ webhookSecret }}</code>
          </p>

          <h3>Mini App 등록</h3>
          <form @submit.prevent="registerMiniApp">
            <input v-model="miniAppOrigin" placeholder="https://miniapp.example.com" required />
            <button>MiniApp 등록</button>
          </form>
        </div>
      </section>
    </template>
  </main>
</template>

<style>
body{background:#f5f7fb;font-family:system-ui;margin:0}main{max-width:900px;margin:35px auto;padding:30px;background:#fff;border-radius:12px}input,button{padding:10px;border:1px solid #ddd;border-radius:8px;margin:5px}button{background:#165dd8;color:white;cursor:pointer}section{padding:20px;border:1px solid #ddd;border-radius:12px;margin:14px 0}.app-card{padding:10px;margin:8px 0;background:#f8fafc;border:1px solid #e2e8f0;border-radius:8px;cursor:pointer}.app-card.active,.active{border-color:#165dd8;background:#ebf3ff}.tool-box{margin-top:16px;padding:12px;border-top:1px solid #eee}.error{color:#a22;background:#ffebeb;padding:10px;border-radius:8px}.note{color:#076e3b;background:#eaf8ef;padding:10px;border-radius:8px}.secret{padding:12px;background:#fff4d8;border-radius:8px;overflow-wrap:anywhere}
</style>
