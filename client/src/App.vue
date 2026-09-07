<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { deviceId, request } from './api'

type Conversation = { id: string; name: string; kind: string }
type Message = { id: string; senderId: string; content: string; retractedAt?: string; createdAt: string }
const brand = ref({ companyName: '公司', appName: '企业内网聊天', logo: './logo.svg', primaryColor: '#356AE6' })
const loggedIn = ref(!!localStorage.getItem('token'))
const username = ref(''), password = ref(''), error = ref('')
const conversations = ref<Conversation[]>([]), active = ref<Conversation>(), messages = ref<Message[]>([]), draft = ref('')
const server = ref(localStorage.getItem('server') || 'http://127.0.0.1:8080')

onMounted(async () => { brand.value = await fetch('./brand.json').then(r => r.json()); document.documentElement.style.setProperty('--primary', brand.value.primaryColor); if (loggedIn.value) await loadConversations() })
async function login() { try { const data = await request<{token:string}>('/api/v1/auth/login',{method:'POST',body:JSON.stringify({username:username.value,password:password.value,deviceId:deviceId()})});localStorage.setItem('token',data.token);loggedIn.value=true;error.value='';await loadConversations() } catch(e) { error.value=(e as Error).message } }
async function loadConversations(){conversations.value=await request('/api/v1/conversations')}
async function open(c:Conversation){active.value=c;messages.value=(await request<Message[]>(`/api/v1/conversations/${c.id}/messages`)).reverse()}
async function send(){if(!active.value||!draft.value.trim())return;const m=await request<Message>(`/api/v1/conversations/${active.value.id}/messages`,{method:'POST',body:JSON.stringify({type:'text',content:draft.value})});messages.value.push(m);draft.value=''}
function logout(){localStorage.removeItem('token');loggedIn.value=false;active.value=undefined;messages.value=[]}
function saveServer(){ localStorage.setItem('server', server.value); login() }
</script>

<template>
  <main v-if="!loggedIn" class="login-shell">
    <section class="login-card"><img :src="brand.logo" alt="公司标志"><small>{{ brand.companyName }}</small><h1>{{ brand.appName }}</h1><p>连接公司内网，开始安全沟通</p><input v-model="server" placeholder="服务器地址，例如 http://192.168.1.100:8080"><input v-model="username" placeholder="用户名" @keyup.enter="login"><input v-model="password" type="password" placeholder="密码" @keyup.enter="login"><button @click="saveServer">登录</button><div class="error">{{ error }}</div></section>
  </main>
  <main v-else class="workspace">
    <aside class="rail"><img :src="brand.logo"><button title="聊天">聊</button><button title="通讯录">人</button><button title="文件">件</button><span></span><button @click="logout" title="退出">退</button></aside>
    <aside class="conversation-list"><header><strong>消息</strong><button>＋</button></header><input placeholder="搜索会话"><button v-for="c in conversations" :key="c.id" class="conversation" :class="{selected:active?.id===c.id}" @click="open(c)"><span>{{ c.kind==='group'?'群':'聊' }}</span><div><strong>{{ c.name || '未命名会话' }}</strong><small>点击查看消息</small></div></button><div v-if="!conversations.length" class="empty">暂无会话</div></aside>
    <section class="chat"><header><div><strong>{{ active?.name || '选择一个会话' }}</strong><small v-if="active">内网连接正常</small></div><button>•••</button></header><div class="messages"><div v-if="!active" class="welcome"><img :src="brand.logo"><h2>{{ brand.appName }}</h2><p>从左侧选择会话开始聊天</p></div><article v-for="m in messages" :key="m.id"><div class="avatar">员</div><div><small>{{ new Date(m.createdAt).toLocaleString() }}</small><p>{{ m.retractedAt ? '消息已撤回' : m.content }}</p></div></article></div><footer v-if="active"><div class="tools">☺　📎　▣</div><textarea v-model="draft" placeholder="输入消息，Enter 发送" @keydown.enter.prevent="send"></textarea><button @click="send">发送</button></footer></section>
  </main>
</template>
