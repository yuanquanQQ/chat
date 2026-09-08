<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { deviceId, fileUrl, request } from './api'

type Conversation = { id: string; name: string; kind: string }
type Message = {
  id: string; senderId: string; type?: string; content: string
  replyToId?: string | null; replyToContent?: string
  metadata?: any; retractedAt?: string; createdAt: string
  readByOther?: boolean | null; readCount?: number
}
type User = { id: string; username: string; realName: string; department: string }
type PendingUser = User & { createdAt: string }
type DeptRequest = { id: string; username: string; realName: string; targetDepartment: string; reason: string; createdAt: string }
type AdminUser = User & { status: string; createdAt: string }
type GroupMember = { id: string; username: string; realName: string; role: string }
type SearchHit = { id: string; conversationId: string; conversationName: string; senderName: string; content: string; createdAt: string }

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
const myId = ref(localStorage.getItem('uid') || '')

const panel = ref<'contacts' | 'admin' | 'deptReq' | 'group' | 'search' | null>(null)
const adminTab = ref<'pending' | 'dept' | 'users'>('pending')
const pendingUsers = ref<PendingUser[]>([])
const contacts = ref<User[]>([])
const deptRequests = ref<DeptRequest[]>([])
const adminUsers = ref<AdminUser[]>([])
const groupMembers = ref<GroupMember[]>([])
const groupName = ref('')
const deptForm = reactive({ department: '', reason: '' })
const deptMsg = ref('')

// 文件 / 表情 / 引用 / 预览 / 搜索 / 未读
const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const showEmoji = ref(false)
const emojis = ['😀', '😄', '😂', '🤣', '😊', '😍', '😘', '😎', '🤔', '😴', '😅', '😭', '😡', '🥳', '👍', '👎', '👏', '🙏', '💪', '🔥', '❤️', '🎉', '✅', '❌', '🤝', '👀', '💯', '🙌', '🎈', '⭐']
const replyTo = ref<Message | null>(null)
const hoverMsg = ref('')
const preview = ref<{ id: string; name: string; mime: string } | null>(null)
const searchQ = ref('')
const searchResults = ref<SearchHit[]>([])
const unreadCount = ref<Record<string, number>>({})
const loadingOlder = ref(false)
const newGroupVisible = ref(false)
const newGroupName = ref('')

let ws: WebSocket | null = null
let wsTimer: ReturnType<typeof setTimeout> | null = null

// ---------- WebSocket 实时 ----------
function connectWS() {
  if (!loggedIn.value) return
  const token = localStorage.getItem('token') || ''
  const srv = localStorage.getItem('server') || 'http://127.0.0.1:8080'
  const url = srv.replace(/^http/i, 'ws') + '/ws'
  try { ws = new WebSocket(url, ['bearer', token]) } catch { scheduleReconnect(); return }
  ws.onmessage = (ev) => { try { handleWSEvent(JSON.parse(ev.data as string)) } catch { /* ignore */ } }
  ws.onclose = () => { ws = null; scheduleReconnect() }
  ws.onerror = () => { try { ws?.close() } catch { /* ignore */ } }
}
function scheduleReconnect() { if (!loggedIn.value || wsTimer) return; wsTimer = setTimeout(() => { wsTimer = null; connectWS() }, 3000) }
function handleWSEvent(ev: { type: string; data: any; conversationId: string }) {
  if (ev.type === 'message.created') {
    const m = ev.data as Message
    if (active.value && active.value.id === ev.conversationId && !messages.value.find(x => x.id === m.id)) {
      messages.value.push(m)
    } else if (active.value?.id !== ev.conversationId) {
      unreadCount.value[ev.conversationId] = (unreadCount.value[ev.conversationId] || 0) + 1
    }
    loadConversations()
  } else if (ev.type === 'message.retracted') {
    const m = messages.value.find(x => x.id === ev.data?.id)
    if (m) m.retractedAt = new Date().toISOString()
  } else if (ev.type === 'conversation.updated') {
    loadConversations()
  }
}

// ---------- 生命周期 ----------
onMounted(async () => {
  brand.value = await fetch('./brand.json').then(r => r.json())
  document.documentElement.style.setProperty('--primary', brand.value.primaryColor)
  if (loggedIn.value) {
    try {
      const me = await request<{ id: string }>('/api/v1/me')
      localStorage.setItem('uid', me.id)
      myId.value = me.id
    } catch { logout(); return }
    await loadConversations()
    connectWS()
  }
})
onBeforeUnmount(() => { try { ws?.close() } catch { /* ignore */ } })

// ---------- 登录 / 注册 ----------
async function login() {
  try {
    const data = await request<{ token: string, user: { id: string, role: string } }>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ username: username.value, password: password.value, deviceId: deviceId() }) })
    localStorage.setItem('token', data.token); localStorage.setItem('role', data.user.role); localStorage.setItem('uid', data.user.id)
    isAdmin.value = data.user.role === 'admin'; myId.value = data.user.id; loggedIn.value = true; error.value = ''
    await loadConversations(); connectWS()
  } catch (e) { error.value = (e as Error).message }
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
function logout() {
  try { ws?.close() } catch { /* ignore */ }
  localStorage.removeItem('token'); localStorage.removeItem('role'); localStorage.removeItem('uid')
  loggedIn.value = false; isAdmin.value = false; panel.value = null
  pendingUsers.value = []; contacts.value = []; deptRequests.value = []; adminUsers.value = []; groupMembers.value = []
  active.value = undefined; messages.value = []
}
function saveServer() { localStorage.setItem('server', server.value); login() }

// ---------- 面板切换 ----------
async function togglePanel(name: 'contacts' | 'admin' | 'deptReq') {
  if (panel.value === name) { panel.value = null; return }
  panel.value = name
  if (name === 'contacts') await loadContacts()
  else if (name === 'admin') { if (adminTab.value === 'pending') await loadPending(); else if (adminTab.value === 'dept') await loadDeptRequests(); else await loadAdminUsers() }
}
async function switchAdminTab(tab: 'pending' | 'dept' | 'users') {
  adminTab.value = tab
  if (tab === 'pending') await loadPending()
  else if (tab === 'dept') await loadDeptRequests()
  else await loadAdminUsers()
}

// ---------- 通讯录 ----------
async function loadContacts() { contacts.value = await request('/api/v1/users') }
async function openDirect(u: User) {
  const conv = await request<Conversation>('/api/v1/conversations', { method: 'POST', body: JSON.stringify({ kind: 'direct', memberIDs: [u.id] }) })
  conv.name = u.realName
  let c = conversations.value.find(x => x.id === conv.id)
  if (!c) { conversations.value.unshift(conv); c = conv }
  panel.value = null
  await open(c)
}

// ---------- 审核（admin） ----------
async function loadPending() { pendingUsers.value = await request('/api/v1/admin/users/pending') }
async function approveUser(id: string) { await request(`/api/v1/admin/users/${id}/approve`, { method: 'POST' }); await loadPending() }
async function rejectUser(id: string) { await request(`/api/v1/admin/users/${id}/reject`, { method: 'POST' }); await loadPending() }

// ---------- 部门申请 / 审批 ----------
async function submitDeptRequest() {
  deptMsg.value = ''
  try {
    await request('/api/v1/me/department-request', { method: 'POST', body: JSON.stringify({ department: deptForm.department, reason: deptForm.reason }) })
    deptMsg.value = '已提交申请，请等待管理员审批'
    deptForm.department = ''; deptForm.reason = ''
  } catch (e) { deptMsg.value = (e as Error).message }
}
async function loadDeptRequests() { deptRequests.value = await request('/api/v1/admin/department-requests') }
async function approveDeptReq(id: string) { await request(`/api/v1/admin/department-requests/${id}/approve`, { method: 'POST' }); await loadDeptRequests() }
async function rejectDeptReq(id: string) { await request(`/api/v1/admin/department-requests/${id}/reject`, { method: 'POST' }); await loadDeptRequests() }

// ---------- 用户管理（admin） ----------
async function loadAdminUsers() { adminUsers.value = await request('/api/v1/admin/users') }
async function adminDisable(id: string) { await request(`/api/v1/admin/users/${id}/disable`, { method: 'POST' }); await loadAdminUsers() }
async function adminEnable(id: string) { await request(`/api/v1/admin/users/${id}/enable`, { method: 'POST' }); await loadAdminUsers() }
async function adminResetPwd(u: AdminUser) {
  const pwd = window.prompt(`为用户 ${u.realName} (@${u.username}) 设置新密码（至少 8 位）`)
  if (!pwd) return
  try { await request(`/api/v1/admin/users/${u.id}/reset-password`, { method: 'POST', body: JSON.stringify({ newPassword: pwd }) }); window.alert('密码已重置') }
  catch (e) { window.alert((e as Error).message) }
}

// ---------- 会话 ----------
async function createGroup() {
  if (!newGroupName.value.trim()) return
  const conv = await request<Conversation>('/api/v1/conversations', { method: 'POST', body: JSON.stringify({ kind: 'group', name: newGroupName.value, memberIDs: [] }) })
  conversations.value.unshift(conv)
  newGroupName.value = ''; newGroupVisible.value = false
  await open(conv)
}
async function loadConversations() { conversations.value = await request('/api/v1/conversations') }
async function open(c: Conversation) {
  panel.value = null; active.value = c
  unreadCount.value[c.id] = 0
  messages.value = (await request<Message[]>(`/api/v1/conversations/${c.id}/messages`)).reverse()
  replyTo.value = null
}
async function loadOlder() {
  if (!active.value || loadingOlder.value || !messages.value.length) return
  loadingOlder.value = true
  try {
    const before = messages.value[0].createdAt
    const older = await request<Message[]>(`/api/v1/conversations/${active.value.id}/messages?before=${encodeURIComponent(before)}`)
    if (older.length) messages.value = [...older.reverse(), ...messages.value]
  } finally { loadingOlder.value = false }
}
async function send() {
  if (!active.value || !draft.value.trim()) return
  const m = await request<Message>(`/api/v1/conversations/${active.value.id}/messages`, { method: 'POST', body: JSON.stringify({ type: 'text', content: draft.value, replyToId: replyTo.value?.id, metadata: {} }) })
  messages.value.push(m); draft.value = ''; replyTo.value = null
}

// ---------- 文件 ----------
function pickFile() { fileInput.value?.click() }
async function onFileSelected(ev: Event) {
  const input = ev.target as HTMLInputElement
  const f = input.files?.[0]
  input.value = ''
  if (!f || !active.value) return
  uploading.value = true
  try {
    if (f.size < 100 * 1024 * 1024) {
      const fd = new FormData()
      fd.append('conversationId', active.value.id)
      fd.append('file', f, f.name)
      const up = await request<{ id: string; mimeType: string; name: string; sizeBytes: number }>('/api/v1/files/uploads', { method: 'POST', body: fd })
      await sendFileMsg(up.id, up.name, up.mimeType, 'server', up.sizeBytes)
    } else {
      const prep = await request<{ id: string }>('/api/v1/files/prepare', { method: 'POST', body: JSON.stringify({ conversationId: active.value.id, name: f.name, mimeType: f.type || 'application/octet-stream', sizeBytes: f.size, clientLocator: 'local' }) })
      await sendFileMsg(prep.id, f.name, f.type || 'application/octet-stream', 'client', f.size)
    }
  } catch (e) { window.alert((e as Error).message) }
  finally { uploading.value = false }
}
async function sendFileMsg(fileId: string, fname: string, mime: string, storageMode: string, size: number) {
  if (!active.value) return
  const typ = mime.startsWith('image/') ? 'image' : mime.startsWith('video/') ? 'video' : mime.startsWith('audio/') ? 'audio' : 'file'
  const m = await request<Message>(`/api/v1/conversations/${active.value.id}/messages`, { method: 'POST', body: JSON.stringify({ type: typ, content: fname, replyToId: replyTo.value?.id, metadata: { fileId, fileName: fname, fileSize: size, mimeType: mime, storageMode } }) })
  messages.value.push(m); replyTo.value = null
}
async function storeToServer(m: Message) {
  if (!active.value || !m.metadata?.fileId) return
  const fd = new FormData()
  fd.append('file', new Blob(), 'placeholder') // 真实客户端模式上传实体在本轮由发送者手动提供；此处提示
  try {
    const data = await request<{ storageMode: string }>(`/api/v1/files/${m.metadata.fileId}/store-to-server`, { method: 'POST', body: fd })
    if (data.storageMode === 'server') { m.metadata.storageMode = 'server'; window.alert('已转存到服务器') }
  } catch (e) { window.alert((e as Error).message) }
}
function fmtSize(n: number) {
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB'
  return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

// ---------- 表情 / 引用 / 复制 ----------
function insertEmoji(e: string) { draft.value += e }
function copyMsg(m: Message) { navigator.clipboard?.writeText(m.content).then(() => window.alert('已复制')) }
function fmtContent(m: Message) { return m.retractedAt ? '消息已撤回' : m.content }

// ---------- 搜索 ----------
async function search() {
  const q = searchQ.value.trim()
  if (!q) return
  searchResults.value = await request(`/api/v1/search?q=${encodeURIComponent(q)}`)
  panel.value = 'search'
}
async function jumpToHit(h: SearchHit) {
  const conv = conversations.value.find(c => c.id === h.conversationId)
  if (conv) { panel.value = null; await open(conv) }
  else { const c: Conversation = { id: h.conversationId, name: h.conversationName || '会话', kind: 'group' }; conversations.value.unshift(c); await open(c) }
}

// ---------- 群管理 ----------
async function openGroupPanel() {
  if (!active.value || active.value.kind !== 'group') return
  panel.value = 'group'
  groupName.value = active.value.name || ''
  await loadGroupMembers()
}
async function loadGroupMembers() {
  if (!active.value) return
  groupMembers.value = await request(`/api/v1/conversations/${active.value.id}/members`)
}
async function renameGroup() {
  if (!active.value || !groupName.value.trim()) return
  await request(`/api/v1/conversations/${active.value.id}`, { method: 'PATCH', body: JSON.stringify({ name: groupName.value }) })
  active.value.name = groupName.value
  await loadConversations(); panel.value = null
}
async function inviteMember(u: User) {
  if (!active.value) return
  await request(`/api/v1/conversations/${active.value.id}/members`, { method: 'POST', body: JSON.stringify({ userIDs: [u.id] }) })
  await loadGroupMembers()
}
async function kickMember(m: GroupMember) {
  if (!active.value) return
  await request(`/api/v1/conversations/${active.value.id}/members/${m.id}`, { method: 'DELETE' })
  await loadGroupMembers()
}
async function leaveGroup() {
  if (!active.value) return
  await request(`/api/v1/conversations/${active.value.id}/members/${myId.value}`, { method: 'DELETE' })
  await loadConversations(); panel.value = null; active.value = undefined; messages.value = []
}
async function disbandGroup() {
  if (!active.value) return
  if (!window.confirm('确定解散该群聊？所有成员将失去访问，且不可恢复。')) return
  await request(`/api/v1/conversations/${active.value.id}`, { method: 'DELETE' })
  await loadConversations(); panel.value = null; active.value = undefined; messages.value = []
}
const myGroupRole = () => groupMembers.value.find(m => m.id === myId.value)?.role || ''
const statusText: Record<string, string> = { pending: '待审核', active: '正常', rejected: '已拒绝', disabled: '已禁用' }
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
    <aside class="rail"><img :src="brand.logo"><button title="聊天" @click="panel = null">聊</button><button title="通讯录" @click="togglePanel('contacts')">人</button><button title="转部门" @click="togglePanel('deptReq')">部</button><button v-if="isAdmin" title="管理" @click="togglePanel('admin')">审</button><span></span><button @click="logout" title="退出">退</button></aside>
    <aside class="conversation-list">
      <header><strong>消息</strong><button title="新建群聊" @click="newGroupVisible = !newGroupVisible">＋</button></header>
      <input v-if="newGroupVisible" v-model="newGroupName" placeholder="群聊名称，回车创建" @keyup.enter="createGroup">
      <input v-model="searchQ" placeholder="搜索聊天记录，回车搜索" @keyup.enter="search">
      <button v-for="c in conversations" :key="c.id" class="conversation" :class="{selected:active?.id===c.id}" @click="open(c)">
        <span>{{ c.kind==='group'?'群':'聊' }}</span><div><strong>{{ c.name || '未命名会话' }}</strong><small>点击查看消息</small></div><b v-if="unreadCount[c.id]" class="badge">{{ unreadCount[c.id] }}</b>
      </button>
      <div v-if="!conversations.length" class="empty">暂无会话</div>
    </aside>
    <section class="chat">
      <header><div><strong>{{ active?.name || '选择一个会话' }}</strong><small v-if="active">内网连接正常</small></div><button v-if="active?.kind==='group'" title="群管理" @click="openGroupPanel">•••</button></header>
      <div class="messages">
        <div v-if="!active && !panel" class="welcome"><img :src="brand.logo"><h2>{{ brand.appName }}</h2><p>从左侧选择会话开始聊天</p></div>

        <!-- 管理面板 -->
        <div v-if="panel === 'admin'" class="pending-panel">
          <div class="tabs"><button :class="{tabon: adminTab==='pending'}" @click="switchAdminTab('pending')">待审核用户</button><button :class="{tabon: adminTab==='dept'}" @click="switchAdminTab('dept')">部门申请</button><button :class="{tabon: adminTab==='users'}" @click="switchAdminTab('users')">用户管理</button></div>
          <template v-if="adminTab === 'pending'"><h3>待审核用户（{{ pendingUsers.length }}）</h3><div v-if="!pendingUsers.length" class="empty">没有待审核用户</div><article v-for="u in pendingUsers" :key="u.id"><div class="pu-info"><strong>{{ u.realName }}</strong><small>@{{ u.username }} · {{ u.department }} · {{ new Date(u.createdAt).toLocaleString() }}</small></div><div class="row-actions"><button class="pu-approve" @click="approveUser(u.id)">通过</button><button class="pu-reject" @click="rejectUser(u.id)">拒绝</button></div></article></template>
          <template v-if="adminTab === 'dept'"><h3>部门变更申请（{{ deptRequests.length }}）</h3><div v-if="!deptRequests.length" class="empty">没有待审批的申请</div><article v-for="r in deptRequests" :key="r.id"><div class="pu-info"><strong>{{ r.realName }} → {{ r.targetDepartment }}</strong><small>@{{ r.username }} · 理由：{{ r.reason }} · {{ new Date(r.createdAt).toLocaleString() }}</small></div><div class="row-actions"><button class="pu-approve" @click="approveDeptReq(r.id)">批准</button><button class="pu-reject" @click="rejectDeptReq(r.id)">拒绝</button></div></article></template>
          <template v-if="adminTab === 'users'"><h3>全部用户（{{ adminUsers.length }}）</h3><article v-for="u in adminUsers" :key="u.id"><div class="pu-info"><strong>{{ u.realName }}</strong><small>@{{ u.username }} · {{ u.department || '未分配' }} · {{ statusText[u.status] || u.status }}</small></div><div class="row-actions"><button class="pu-approve" @click="adminResetPwd(u)">重置密码</button><button v-if="u.status==='active'" class="pu-reject" @click="adminDisable(u.id)">禁用</button><button v-if="u.status==='disabled'" class="pu-approve" @click="adminEnable(u.id)">启用</button></div></article></template>
        </div>

        <!-- 通讯录 -->
        <div v-if="panel === 'contacts'" class="pending-panel"><h3>公司通讯录（{{ contacts.length }}）</h3><div v-if="!contacts.length" class="empty">通讯录为空</div><article v-for="u in contacts" :key="u.id" class="contact" :class="{self: u.id === myId}" @click="u.id !== myId && openDirect(u)"><div class="pu-info"><strong>{{ u.realName }}<span v-if="u.id === myId" class="me-tag">（我）</span></strong><small>@{{ u.username }}{{ u.department ? ' · ' + u.department : '' }}</small></div><span class="pu-approve">{{ u.id === myId ? '' : '发起聊天 →' }}</span></article></div>

        <!-- 转部门 -->
        <div v-if="panel === 'deptReq'" class="pending-panel"><h3>申请转部门</h3><input v-model="deptForm.department" placeholder="目标部门"><textarea v-model="deptForm.reason" placeholder="申请理由" rows="3"></textarea><button class="pu-approve wide" @click="submitDeptRequest">提交申请</button><div class="ok">{{ deptMsg }}</div></div>

        <!-- 搜索 -->
        <div v-if="panel === 'search'" class="pending-panel"><h3>搜索结果（{{ searchResults.length }}）</h3><div v-if="!searchResults.length" class="empty">没有匹配的消息</div><article v-for="h in searchResults" :key="h.id" class="contact" @click="jumpToHit(h)"><div class="pu-info"><strong>{{ h.senderName }} @ {{ h.conversationName || '会话' }}</strong><small>{{ h.content }} · {{ new Date(h.createdAt).toLocaleString() }}</small></div><span class="pu-approve">打开 →</span></article></div>

        <!-- 群管理 -->
        <div v-if="panel === 'group'" class="pending-panel">
          <h3>群管理</h3>
          <div class="grp-row"><input v-model="groupName" placeholder="群名称"><button class="pu-approve" @click="renameGroup">保存名称</button></div>
          <h4>成员（{{ groupMembers.length }}）</h4>
          <article v-for="m in groupMembers" :key="m.id"><div class="pu-info"><strong>{{ m.realName }}<span v-if="m.id === myId" class="me-tag">（我）</span></strong><small>@{{ m.username }} · {{ m.role === 'owner' ? '群主' : m.role === 'admin' ? '管理员' : '成员' }}</small></div><button v-if="m.id !== myId && (myGroupRole()==='owner'||myGroupRole()==='admin')" class="pu-reject" @click="kickMember(m)">移除</button></article>
          <h4>邀请成员</h4>
          <article v-for="u in contacts.filter(x => !groupMembers.some(m => m.id === x.id))" :key="u.id" class="contact"><div class="pu-info"><strong>{{ u.realName }}</strong><small>@{{ u.username }}{{ u.department ? ' · ' + u.department : '' }}</small></div><button class="pu-approve" @click="inviteMember(u)">邀请</button></article>
          <div class="grp-actions"><button v-if="myGroupRole() !== 'owner'" class="pu-reject" @click="leaveGroup">退出群聊</button><button v-if="myGroupRole() === 'owner'" class="pu-reject" @click="disbandGroup">解散群聊</button></div>
        </div>

        <!-- 消息列表 -->
        <template v-if="!panel">
          <div v-if="messages.length" class="load-older"><button @click="loadOlder">{{ loadingOlder ? '加载中...' : '加载更早的消息' }}</button></div>
          <article v-for="m in messages" :key="m.id" :class="['msg', m.senderId === myId ? 'mine' : 'theirs']" @mouseenter="hoverMsg = m.id" @mouseleave="hoverMsg = ''">
            <div v-if="m.senderId !== myId" class="avatar">员</div>
            <div class="bubble">
              <small>{{ new Date(m.createdAt).toLocaleString() }}</small>
              <div v-if="m.replyToId" class="reply-preview">{{ m.replyToContent ? '引用：' + m.replyToContent.slice(0, 60) : '引用了一条消息' }}</div>
              <template v-if="m.retractedAt"><p>消息已撤回</p></template>
              <template v-else-if="m.type === 'image' && m.metadata?.fileId"><img class="msg-media" :src="fileUrl(m.metadata.fileId)" alt="图片" @click="preview = { id: m.metadata.fileId, name: m.content, mime: m.metadata.mimeType }"><p class="media-name">{{ m.content }}</p></template>
              <template v-else-if="m.type === 'video' && m.metadata?.fileId"><video class="msg-media" :src="fileUrl(m.metadata.fileId)" controls></video><p class="media-name">{{ m.content }}</p></template>
              <template v-else-if="m.type === 'audio' && m.metadata?.fileId"><audio :src="fileUrl(m.metadata.fileId)" controls></audio><p class="media-name">{{ m.content }}</p></template>
              <template v-else-if="m.type === 'file' && m.metadata?.fileId">
                <div class="file-card"><div class="file-icon">📄</div><div class="file-info"><strong>{{ m.metadata.fileName || m.content }}</strong><small>{{ fmtSize(m.metadata.fileSize || 0) }} · {{ m.metadata.storageMode === 'client' ? '发送者客户端' : '服务器' }}</small></div>
                  <div class="file-actions"><a :href="fileUrl(m.metadata.fileId)" v-if="m.metadata.storageMode === 'server'" download>下载</a><button v-if="m.metadata.storageMode === 'client'" @click="storeToServer(m)">转存服务器</button></div>
                </div>
              </template>
              <template v-else><p>{{ fmtContent(m) }}</p></template>
              <div v-if="m.senderId === myId && m.type === 'text' && !m.retractedAt" class="read-state">{{ m.readByOther === true ? '已读 ✓' : m.readByOther === false ? '未读' : '' }}</div>
              <div v-if="m.type === 'text' && !m.retractedAt && active?.kind === 'group' && m.readCount" class="read-state">已读 {{ m.readCount }}</div>
              <div v-if="hoverMsg === m.id" class="msg-actions"><button @click="replyTo = m">引用</button><button @click="copyMsg(m)">复制</button></div>
            </div>
            <div v-if="m.senderId === myId" class="avatar me">我</div>
          </article>
        </template>
      </div>
      <footer v-if="active">
        <div v-if="replyTo" class="reply-bar">回复 {{ replyTo.senderId === myId ? '自己' : '对方' }}：{{ replyTo.content.slice(0, 40) }}<button @click="replyTo = null">✕</button></div>
        <div class="tools">
          <button title="表情" @click="showEmoji = !showEmoji">☺</button>
          <button title="发送文件" @click="pickFile" :disabled="uploading">{{ uploading ? '上传中...' : '📎' }}</button>
          <input ref="fileInput" type="file" style="display:none" @change="onFileSelected">
          <div v-if="showEmoji" class="emoji-panel"><button v-for="e in emojis" :key="e" @click="insertEmoji(e)">{{ e }}</button></div>
        </div>
        <textarea v-model="draft" placeholder="输入消息，Enter 发送" @keydown.enter.prevent="send"></textarea><button @click="send">发送</button>
      </footer>
    </section>
    <!-- 预览弹窗 -->
    <div v-if="preview" class="preview-overlay" @click.self="preview = null">
      <div class="preview-box">
        <button class="preview-close" @click="preview = null">✕</button>
        <strong>{{ preview.name }}</strong>
        <img v-if="preview.mime?.startsWith('image/')" :src="fileUrl(preview.id)" alt="预览">
        <video v-else-if="preview.mime?.startsWith('video/')" :src="fileUrl(preview.id)" controls autoplay></video>
        <audio v-else-if="preview.mime?.startsWith('audio/')" :src="fileUrl(preview.id)" controls></audio>
        <iframe v-else :src="fileUrl(preview.id)" class="preview-frame"></iframe>
        <a :href="fileUrl(preview.id)" download>下载</a>
      </div>
    </div>
  </main>
</template>
