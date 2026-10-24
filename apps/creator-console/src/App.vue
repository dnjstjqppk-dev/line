<script setup lang="ts">
import {ref} from 'vue'
import {request,type LoginResult} from '../../shared/api'
const token=ref(''),email=ref(''),password=ref(''),profile=ref(''),title=ref(''),price=ref(0),error=ref(''),message=ref('')
const packs=ref<Array<{id:string;title:string;status:string;price_minor:number;currency:string}>>([])
async function run(fn:()=>Promise<void>){error.value='';message.value='';try{await fn()}catch(e){error.value=(e as Error).message}}
async function login(){await run(async()=>{const x=await request<LoginResult>('/api/v1/auth/login',null,'POST',{email:email.value,password:password.value});token.value=x.access_token;password.value='';await refresh()})}
async function refresh(){packs.value=(await request<{packs:typeof packs.value}>('/api/v1/creator/packs',token.value)).packs}
async function saveProfile(){await run(async()=>{await request('/api/v1/creator/profile',token.value,'POST',{display_name:profile.value});message.value='크리에이터 프로필 생성 완료'})}
async function createPack(){await run(async()=>{await request('/api/v1/creator/packs',token.value,'POST',{title:title.value,price_minor:price.value,currency:'KRW'});title.value='';await refresh();message.value='스티커 팩 초안 생성 완료'})}
</script><template><main><h1>Creator Center</h1><p>크리에이터 프로필과 스티커 팩 초안 관리 — 이미지 업로드·심사·판매·정산 미구현</p><p role="alert" class="error" v-if="error">{{error}}</p><p class="note" v-if="message">{{message}}</p>
<form v-if="!token" @submit.prevent="login"><input type="email" v-model="email" placeholder="이메일" required/><input v-model="password" type="password" placeholder="비밀번호" required/><button>로그인</button></form>
<template v-else><section><h2>크리에이터 프로필</h2><form @submit.prevent="saveProfile"><input v-model="profile" placeholder="표시 이름" minlength="2" required/><button>등록 / 수정</button></form></section><section><h2>새 스티커 팩</h2><form @submit.prevent="createPack"><input v-model="title" placeholder="팩 제목" minlength="2" required/><input v-model.number="price" type="number" min="0" placeholder="가격 (원)"/><button>초안 저장</button></form></section><section><h2>내 스티커 팩</h2><p v-for="p in packs" :key="p.id">{{p.title}} · {{p.status}} · {{p.price_minor}} {{p.currency}}</p></section></template></main></template>
<style>body{font:16px system-ui;background:#f4f6fa;color:#172536}main{max-width:850px;margin:35px auto;background:white;padding:30px;border-radius:12px}section{border:1px solid #ddd;padding:20px;border-radius:10px;margin:16px 0}input,button{padding:11px;border:1px solid #ccc;border-radius:8px;margin:5px}button{background:#096b52;color:white;cursor:pointer}.error{color:#a22}.note{color:#176b3e}</style>
