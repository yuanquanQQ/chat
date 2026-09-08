<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { deviceId, request } from './api'

type Conversation = { id: string; name: string; kind: string }
type Message = { id: string; senderId: string; content: string; retractedAt?: string; createdAt: string }
const brand = ref({ companyName: '公司', appName: '企业内网聊天', logo: './logo.svg', primaryColor: '#356AE6' })
const loggedIn = ref(!!localStorage.getItem('token'))
const username = ref(''), password = ref(''), error = ref('')
const conversations = ref<Conversation[]>([]), active = ref<Conversation>(), messages = ref<Message[]>([]), draft = ref('')
const server = ref(localStorage.getItem('server') || 'http://127.0.0.1:8080')
const mode = ref<'login' | 'register'>('login')
const reg = reactive({ username: '', password: '', realName: '', avatarUrl: '', department: '' })
const regError = ref('')
const regDone = ref(false)
const isAdmin = ref(localStorage.getItem('role') === 'admin')
const panel = ref<'contacts' | 'pending' | null>(null)
const pendingUsers = ref<{ id: string; username: string; realName: string; department: string; createdAt: string }[]>([])
const contacts = ref<{ id: string; username: string; realName: string; department: string }[]>([])
const myId = ref(localStorage.getItem('uid') || '')

onMounted(async () => {
  brand.value = await fetch('./brand.json').then(r => r.json())
  document.documentElement.style.setProperty('--primary', brand.value.primaryColor)
  if (loggedIn.value) {
    try {
      // 启动时拉取当前用户，保证旧登录态也能拿到自己的 id（消息方向判断依赖它）
      const me = await request<{ id: string }>('/api/v1/me')
      localStorage.setItem('uid', me.id)
      myId.value = me.id
    } catch { logout() }
    await loadConversations()
  }
})
async function login() { try { const data = await request<{token:string,user:{id:string,role:string}}>('/api/v1/auth/login',{method:'POST',body:JSON.stringify({username:username.value,password:password.value,deviceId:deviceId()})});localStorage.setItem('token',data.token);localStorage.setItem('role',data.user.role);localStorage.setItem('uid',data.user.id);isAdmin.value=data.user.role==='admin';myId.value=data.user.id;loggedIn.value=true;error.value='';await loadConversations() } catch(e) { error.value=(e as Error).message } }
async function togglePanel(name: 'contacts' | 'pending') {
  if (panel.value === name) { panel.value = null; return }
  panel.value = name
  if (name === 'contacts') await loadContacts()
  else await loadPending()
}
async function loadPending() { pendingUsers.value = await request('/api/v1/admin/users/pending') }
async function approveUser(id: string) { await request(`/api/v1/admin/users/${id}/approve`, { method: 'POST' }); await loadPending() }
async function loadContacts() { contacts.value = await request('/api/v1/users') }
async function openDirect(u: { id: string; realName: string }) {
  const conv = await request<Conversation>('/api/v1/conversations', { method: 'POST', body: JSON.stringify({ kind: 'direct', memberIDs: [u.id] }) })
  conv.name = u.realName
  let c = conversations.value.find(x => x.id === conv.id)
  if (!c) { conversations.value.unshift(conv); c = conv }
  panel.value = null
  await open(c)
}
function toRegister() { mode.value = 'register'; regError.value = ''; regDone.value = false; error.value = '' }
function toLogin() { mode.value = 'login'; regError.value = ''; regDone.value = false }
async function register() {
  regError.value = ''
  try {
    await request('/api/v1/auth/register', { method: 'POST', body: JSON.stringify({ username: reg.username, password: reg.password, realName: reg.realName, avatarUrl: reg.avatarUrl.trim() || 'default', department: reg.department }) })
    regDone.value = true
    reg.username = reg.password = reg.realName = reg.avatarUrl = reg.department = ''
  } catch (e) { regError.value = (e as Error).message }
}
async function loadConversations(){conversations.value=await request('/api/v1/conversations')}
async function open(c:Conversation){panel.value=null;active.value=c;messages.value=(await request<Message[]>(`/api/v1/conversations/${c.id}/messages`)).reverse()}
async function send(){if(!active.value||!draft.value.trim())return;const m=await request<Message>(`/api/v1/conversations/${active.value.id}/messages`,{method:'POST',body:JSON.stringify({type:'text',content:draft.value})});messages.value.push(m);draft.value=''}
function logout(){localStorage.removeItem('token');localStorage.removeItem('role');localStorage.removeItem('uid');loggedIn.value=false;isAdmin.value=false;panel.value=null;pendingUsers.value=[];contacts.value=[];active.value=undefined;messages.value=[]}
function saveServer(){ localStorage.setItem('server', server.value); login() }
</script>

<template>
  <main v-if="!loggedIn" class="login-shell">
    <section class="login-card"><img :src="brand.logo" alt="公司标志"><small>{{ brand.companyName }}</small><h1>{{ brand.appName }}</h1><input v-model="server" placeholder="服务器地址，例如 http://192.168.1.100:8080">
      <template v-if="mode === 'login'">
        <p>连接公司内网，开始安全沟通</p><input v-model="username" placeholder="用户名" @keyup.enter="login"><input v-model="password" type="password" placeholder="密码" @keyup.enter="login"><button @click="saveServer">登录</button><div class="error">{{ error }}</div><div class="switch">还没有账号？<button class="link" @click="toRegister">注册新账号</button></div>
      </template>
      <template v-else>
        <p>注册账号，等待管理员审核后即可登录</p><input v-model="reg.username" placeholder="用户名（至少 3 位）"><input v-model="reg.password" type="password" placeholder="密码（至少 8 位）"><input v-model="reg.realName" placeholder="真实姓名"><input v-model="reg.department" placeholder="部门"><input v-model="reg.avatarUrl" placeholder="头像地址（可留空）"><button @click="register">提交注册</button><div v-if="regDone" class="ok">注册成功！请等待管理员审核后登录。</div><div class="error">{{ regError }}</div><div class="switch"><button class="link" @click="toLogin">返回登录</button></div>
      </template>
    </section>
  </main>
  <main v-else class="workspace">
    <aside class="rail"><img :src="brand.logo"><button title="聊天" @click="panel = null">聊</button><button title="通讯录" @click="togglePanel('contacts')">人</button><button v-if="isAdmin" title="待审核用户" @click="togglePanel('pending')">审</button><span></span><button @click="logout" title="退出">退</button></aside>
    <aside class="conversation-list"><header><strong>消息</strong><button>＋</button></header><input placeholder="搜索会话"><button v-for="c in conversations" :key="c.id" class="conversation" :class="{selected:active?.id===c.id}" @click="open(c)"><span>{{ c.kind==='group'?'群':'聊' }}</span><div><strong>{{ c.name || '未命名会话' }}</strong><small>点击查看消息</small></div></button><div v-if="!conversations.length" class="empty">暂无会话</div></aside>
    <section class="chat"><header><div><strong>{{ active?.name || '选择一个会话' }}</strong><small v-if="active">内网连接正常</small></div><button>•••</button></header><div class="messages"><div v-if="!active && !panel" class="welcome"><img :src="brand.logo"><h2>{{ brand.appName }}</h2><p>从左侧选择会话开始聊天</p></div><div v-if="panel === 'pending'" class="pending-panel"><h3>待审核用户（{{ pendingUsers.length }}）</h3><div v-if="!pendingUsers.length" class="empty">没有待审核用户</div><article v-for="u in pendingUsers" :key="u.id"><div class="pu-info"><strong>{{ u.realName }}</strong><small>@{{ u.username }} · {{ u.department }} · {{ new Date(u.createdAt).toLocaleString() }}</small></div><button class="pu-approve" @click="approveUser(u.id)">通过</button></article></div><div v-if="panel === 'contacts'" class="pending-panel"><h3>公司通讯录（{{ contacts.length }}）</h3><div v-if="!contacts.length" class="empty">通讯录为空</div><article v-for="u in contacts" :key="u.id" class="contact" :class="{self: u.id === myId}" @click="u.id !== myId && openDirect(u)"><div class="pu-info"><strong>{{ u.realName }}<span v-if="u.id === myId" class="me-tag">（我）</span></strong><small>@{{ u.username }}{{ u.department ? ' · ' + u.department : '' }}</small></div><span class="pu-approve">{{ u.id === myId ? '' : '发起聊天 →' }}</span></article></div><template v-if="!panel"><article v-for="m in messages" :key="m.id" :class="['msg', m.senderId === myId ? 'mine' : 'theirs']"><div v-if="m.senderId !== myId" class="avatar">员</div><div class="bubble"><small>{{ new Date(m.createdAt).toLocaleString() }}</small><p>{{ m.retractedAt ? '消息已撤回' : m.content }}</p></div><div v-if="m.senderId === myId" class="avatar me">我</div></article></template></div><footer v-if="active"><div class="tools">☺　▣</div><textarea v-model="draft" placeholder="输入消息，Enter 发送" @keydown.enter.prevent="send"></textarea><button @click="send">发送</button></footer></section>
  </main>
</template>
