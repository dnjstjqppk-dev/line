<script setup lang="ts">
import { ref } from 'vue'
import { request, type LoginResult } from '../../shared/api'
type Org={id:string;name:string;state:string;my_role:string};type Account={id:string;name:string;status:string}
const token=ref(''),email=ref(''),password=ref(''),orgName=ref(''),country=ref('KR'),accountName=ref(''),selectedOrg=ref(''),orgs=ref<Org[]>([]),accounts=ref<Account[]>([]),error=ref(''),note=ref('')
async function run(fn:()=>Promise<void>){error.value='';note.value='';try{await fn()}catch(e){error.value=(e as Error).message}}
async function login(){await run(async()=>{const x=await request<LoginResult>('/api/v1/auth/login',null,'POST',{email:email.value,password:password.value});token.value=x.access_token;password.value='';await loadOrgs()})}
async function loadOrgs(){const x=await request<{organizations:Org[]}>('/api/v1/partner/organizations',token.value);orgs.value=x.organizations}
async function createOrg(){await run(async()=>{await request('/api/v1/partner/organizations',token.value,'POST',{name:orgName.value,legal_name:orgName.value,country:country.value});orgName.value='';await loadOrgs();note.value='조직 생성 완료'})}
async function selectOrg(id:string){await run(async()=>{selectedOrg.value=id;const x=await request<{accounts:Account[]}>(`/api/v1/partner/organizations/${id}/accounts`,token.value);accounts.value=x.accounts})}
async function createAccount(){await run(async()=>{await request(`/api/v1/partner/organizations/${selectedOrg.value}/accounts`,token.value,'POST',{name:accountName.value,description:''});accountName.value='';await selectOrg(selectedOrg.value);note.value='공식계정 심사 대기 상태로 생성됨'})}
</script>
<template><div class="wrap"><header><h1>비즈니스 파트너 센터</h1><p>공식계정 · 기업 조직</p></header><p v-if="error" class="error">{{error}}</p><p v-if="note" class="note">{{note}}</p>
<form class="panel" v-if="!token" @submit.prevent="login"><h2>로그인</h2><input v-model="email" type="email" placeholder="이메일" required/><input v-model="password" type="password" placeholder="비밀번호" required/><button>로그인</button></form>
<main v-else class="grid"><section class="panel"><h2>내 기업 조직</h2><form @submit.prevent="createOrg"><input v-model="orgName" placeholder="기업명" minlength="2" required/><input v-model="country" maxlength="8" placeholder="국가 코드"/><button>조직 생성</button></form><div v-for="o in orgs" :key="o.id"><button class="item" @click="selectOrg(o.id)">{{o.name}} <small>({{o.state}} · {{o.my_role}})</small></button></div></section>
<section class="panel"><template v-if="selectedOrg"><h2>공식계정</h2><form @submit.prevent="createAccount"><input v-model="accountName" placeholder="공식계정 이름" minlength="2" required/><button>공식계정 생성</button></form><div class="item" v-for="a in accounts" :key="a.id">{{a.name}} <small>상태: {{a.status}}</small></div></template><p v-else>왼쪽에서 기업 조직을 선택하세요.</p></section>
</main><footer>자동응답·캠페인·CRM·정산 등은 데이터 모델과 API 계약만 준비되었고 아직 실행 기능은 없습니다.</footer></div></template>
<style>body{background:#f3f6fa;margin:0;font:15px system-ui;color:#1d2d45}.wrap{max-width:1100px;margin:auto;padding:25px}.panel{background:white;border-radius:12px;padding:22px;border:1px solid #dce4ed;margin:14px 0}.grid{display:grid;grid-template-columns:1fr 1fr;gap:20px}input,button{padding:11px;border:1px solid #cad5e2;border-radius:9px;margin:5px}button{background:#078452;color:white;cursor:pointer}.item{display:block;margin:8px 0;padding:10px;width:100%;text-align:left;background:#edf3f9;color:#203047}small{color:#6a798b}footer{color:#6a798b}.error{color:#b52222}.note{color:#087743}@media(max-width:600px){.grid{grid-template-columns:1fr}}</style>
