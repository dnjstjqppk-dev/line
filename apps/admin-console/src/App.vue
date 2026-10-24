<script setup lang="ts">
import { ref } from 'vue'
import { request, type LoginResult } from '../../shared/api'
const email=ref(''),password=ref(''),token=ref(''),error=ref('')
const metrics=ref<Record<string,number>>({}),users=ref<Array<Record<string,string>>>([]),reports=ref<Array<Record<string,string>>>([])
async function run(fn:()=>Promise<void>){error.value='';try{await fn()}catch(e){error.value=(e as Error).message}}
async function login(){await run(async()=>{const x=await request<LoginResult>('/api/v1/auth/login',null,'POST',{email:email.value,password:password.value}); token.value=x.access_token;password.value=''; await load()})}
async function load(){await run(async()=>{const [m,u,r]=await Promise.all([request<{metrics:Record<string,number>}>('/api/v1/admin/stats',token.value),request<{users:Array<Record<string,string>>}>('/api/v1/admin/users',token.value),request<{reports:Array<Record<string,string>>}>('/api/v1/admin/reports',token.value)]);metrics.value=m.metrics;users.value=u.users;reports.value=r.reports})}
</script>
<template><div class="page"><header><h1>Messenger 관리자 센터</h1><span>역할: platform_admin · support · moderator (엔드포인트별 구분)</span></header>
<p class="error" v-if="error" role="alert">{{error}}</p>
<form v-if="!token" @submit.prevent="login" class="panel"><h2>관리자 인증</h2><input type="email" v-model="email" placeholder="이메일" required/><input type="password" v-model="password" placeholder="비밀번호" required/><button>로그인</button><p>일반 사용자 계정에는 관리자 권한이 없습니다.</p></form>
<main v-else><button @click="load">다시 조회</button><section class="metrics"><div class="panel" v-for="(value,key) in metrics" :key="key"><small>{{key}}</small><strong>{{value.toLocaleString()}}</strong></div></section>
<section class="panel"><h2>최근 사용자</h2><div class="scroll"><table><thead><tr><th>이름</th><th>이메일</th><th>역할</th><th>상태</th></tr></thead><tbody><tr v-for="u in users" :key="u.id"><td>{{u.display_name}}</td><td>{{u.email}}</td><td>{{u.role}}</td><td>{{u.state}}</td></tr></tbody></table></div></section>
<section class="panel"><h2>신고 목록</h2><p v-if="!reports.length">접수된 신고가 없습니다.</p><div v-for="r in reports" :key="r.id"><strong>{{r.category}}</strong> · {{r.state}} — {{r.details}}</div></section>
<p>제재·수사·콘텐츠 열람 기능은 아직 구현되지 않았습니다. 관리자도 E2EE 내용을 복호화할 수 없습니다.</p>
</main></div></template>
<style>body{margin:0;background:#f2f5fa;color:#15243a;font:15px system-ui,sans-serif}.page{max-width:1150px;margin:auto;padding:28px}header{display:flex;justify-content:space-between;align-items:center;gap:20px}header span{color:#5b6b80}.panel{padding:23px;background:white;margin:18px 0;border:1px solid #dce4f0;border-radius:12px}input,button{padding:11px;border:1px solid #b6c5d5;border-radius:8px;margin:6px}button{background:#225bd6;color:white;cursor:pointer}.metrics{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:12px}.metrics .panel{display:grid;gap:8px}.metrics strong{font-size:30px}.scroll{overflow:auto}table{border-collapse:collapse;width:100%;text-align:left}th,td{padding:9px;border-bottom:1px solid #eef1f5}.error{color:#bc2626}</style>
