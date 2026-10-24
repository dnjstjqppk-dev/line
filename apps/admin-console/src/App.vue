<script setup lang="ts">
import { ref } from 'vue'
import { request, type LoginResult } from '../../shared/api'

const email = ref(''), password = ref(''), token = ref(''), error = ref(''), note = ref('')
const metrics = ref<Record<string, number>>({})
const users = ref<Array<Record<string, string>>>([])
const reports = ref<Array<Record<string, string>>>([])
const auditLogs = ref<Array<Record<string, string>>>([])
const featureFlags = ref<Array<Record<string, unknown>>>([])
const newFlagKey = ref(''), newFlagEnabled = ref(false)

async function run(fn: () => Promise<void>) {
  error.value = ''; note.value = ''
  try { await fn() } catch (e) { error.value = (e as Error).message }
}

async function login() {
  await run(async () => {
    const x = await request<LoginResult>('/api/v1/auth/login', null, 'POST', { email: email.value, password: password.value })
    token.value = x.access_token
    password.value = ''
    await load()
  })
}

async function load() {
  await run(async () => {
    const [m, u, r, a, f] = await Promise.all([
      request<{ metrics: Record<string, number> }>('/api/v1/admin/stats', token.value),
      request<{ users: Array<Record<string, string>> }>('/api/v1/admin/users', token.value),
      request<{ reports: Array<Record<string, string>> }>('/api/v1/admin/reports', token.value),
      request<{ audit_logs: Array<Record<string, string>> }>('/api/v1/admin/audit-logs', token.value),
      request<{ feature_flags: Array<Record<string, unknown>> }>('/api/v1/admin/feature-flags', token.value),
    ])
    metrics.value = m.metrics
    users.value = u.users
    reports.value = r.reports
    auditLogs.value = a.audit_logs
    featureFlags.value = f.feature_flags
  })
}

async function takeAction(subjectUserID: string, action: 'warn' | 'suspend' | 'activate', reportID = '') {
  await run(async () => {
    await request('/api/v1/admin/moderation/actions', token.value, 'POST', {
      subject_user_id: subjectUserID,
      report_id: reportID,
      action,
      reason: `Admin console ${action} action`,
    })
    note.value = `사용자 ${subjectUserID}에 대해 ${action} 조치가 완료되었습니다.`
    await load()
  })
}

async function saveFeatureFlag() {
  if (!newFlagKey.value.trim()) return
  await run(async () => {
    await request('/api/v1/admin/feature-flags', token.value, 'PUT', {
      key: newFlagKey.value.trim(),
      enabled: newFlagEnabled.value,
      config: {},
    })
    newFlagKey.value = ''
    newFlagEnabled.value = false
    note.value = '기능 플래그가 저장되었습니다.'
    await load()
  })
}
</script>

<template>
  <div class="page">
    <header>
      <h1>Messenger 슈퍼 관리자 센터</h1>
      <span>역할: platform_admin · support · moderator</span>
    </header>

    <p class="error" v-if="error" role="alert">{{ error }}</p>
    <p class="note" v-if="note">{{ note }}</p>

    <form v-if="!token" @submit.prevent="login" class="panel">
      <h2>관리자 인증</h2>
      <input type="email" v-model="email" placeholder="이메일" required />
      <input type="password" v-model="password" placeholder="비밀번호" required />
      <button>로그인</button>
      <p class="muted">일반 사용자 계정은 접근이 거부됩니다(403 Forbidden).</p>
    </form>

    <main v-else>
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px">
        <h2>통합 시스템 대시보드</h2>
        <button @click="load">새로고침</button>
      </div>

      <section class="metrics">
        <div class="panel" v-for="(value, key) in metrics" :key="key">
          <small>{{ key }}</small>
          <strong>{{ value.toLocaleString() }}</strong>
        </div>
      </section>

      <section class="panel">
        <h2>사용자 관리 및 제재 집행</h2>
        <div class="scroll">
          <table>
            <thead>
              <tr><th>이름</th><th>이메일</th><th>역할</th><th>상태</th><th>제재 관리</th></tr>
            </thead>
            <tbody>
              <tr v-for="u in users" :key="u.id">
                <td>{{ u.display_name }}</td>
                <td>{{ u.email }}</td>
                <td><span class="badge">{{ u.role }}</span></td>
                <td><span :class="['status', u.state]">{{ u.state }}</span></td>
                <td>
                  <button v-if="u.state === 'active'" style="background:#d9534f;font-size:12px;padding:4px 8px" @click="takeAction(u.id, 'suspend')">정지</button>
                  <button v-else-if="u.state === 'suspended'" style="background:#5cb85c;font-size:12px;padding:4px 8px" @click="takeAction(u.id, 'activate')">정지 해제</button>
                  <button style="background:#f0ad4e;font-size:12px;padding:4px 8px" @click="takeAction(u.id, 'warn')">경고</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="panel">
        <h2>신고 및 악성 콘텐츠 처리</h2>
        <p v-if="!reports.length">접수된 미처리 신고가 없습니다.</p>
        <div v-for="r in reports" :key="r.id" class="report-item">
          <div>
            <strong>[{{ r.category }}]</strong> 상태: {{ r.state }} — {{ r.details }}
          </div>
          <div v-if="r.state === 'open'">
            <button style="background:#d9534f;font-size:12px;padding:4px 8px" @click="takeAction(r.subject_user_id || '', 'suspend', r.id)">피신고자 정지 및 해결</button>
          </div>
        </div>
      </section>

      <section class="panel">
        <h2>글로벌 Feature Flags</h2>
        <form @submit.prevent="saveFeatureFlag" style="display:flex;gap:8px;margin-bottom:12px">
          <input v-model="newFlagKey" placeholder="플래그 키 (예: enable_e2ee, enable_openchat)" required />
          <label style="display:flex;align-items:center;gap:6px">
            <input type="checkbox" v-model="newFlagEnabled" style="width:auto" /> 활성화
          </label>
          <button style="background:#0275d8">플래그 추가/수정</button>
        </form>
        <div v-for="f in featureFlags" :key="String(f.key)" class="item-line">
          <code>{{ f.key }}</code>
          <span :class="['badge', f.enabled ? 'badge-on' : 'badge-off']">{{ f.enabled ? 'ON' : 'OFF' }}</span>
        </div>
      </section>

      <section class="panel">
        <h2>불변 감사 로그 (Audit Logs)</h2>
        <div class="scroll" style="max-height:220px">
          <table>
            <thead><tr><th>시간</th><th>관리자 ID</th><th>작업</th><th>대상 ID</th></tr></thead>
            <tbody>
              <tr v-for="a in auditLogs" :key="a.id">
                <td><small>{{ new Date(a.created_at).toLocaleString() }}</small></td>
                <td><code>{{ a.actor_user_id }}</code></td>
                <td><strong>{{ a.action }}</strong></td>
                <td><code>{{ a.target_id }}</code></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </main>
  </div>
</template>

<style>
body{margin:0;background:#f2f5fa;color:#15243a;font:15px system-ui,sans-serif}.page{max-width:1150px;margin:auto;padding:28px}header{display:flex;justify-content:space-between;align-items:center;gap:20px}header span{color:#5b6b80}.panel{padding:23px;background:white;margin:18px 0;border:1px solid #dce4f0;border-radius:12px}input,button{padding:10px;border:1px solid #b6c5d5;border-radius:8px;margin:4px}button{background:#225bd6;color:white;cursor:pointer}.metrics{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:12px}.metrics .panel{display:grid;gap:8px}.metrics strong{font-size:30px}.scroll{overflow:auto}table{border-collapse:collapse;width:100%;text-align:left}th,td{padding:9px;border-bottom:1px solid #eef1f5}.report-item,.item-line{display:flex;justify-content:space-between;align-items:center;padding:8px 0;border-bottom:1px solid #f0f4f8}.badge{padding:3px 8px;border-radius:6px;font-size:12px;background:#e2e8f0}.badge-on{background:#d4edda;color:#155724}.badge-off{background:#f8d7da;color:#721c24}.status.active{color:#28a745;font-weight:bold}.status.suspended{color:#dc3545;font-weight:bold}.muted{color:#6c757d}.error{color:#bc2626;background:#fde8e8;padding:10px;border-radius:8px}.note{color:#155724;background:#d4edda;padding:10px;border-radius:8px}
</style>
