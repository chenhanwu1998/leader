import { auth, clearSession } from '@/utils/auth'

// —— API 基址 ——
// 浏览器 / 服务模式：同源 '' 即可。
// 桌面应用（Wails 内嵌通道，hostname 为 wails.localhost）：内嵌通道不支持流式响应，
// 桌面端会另起本机回环 HTTP 监听（18520 起连续端口），这里探测出可用端口后直连。
let apiBase = ''

export function getApiBase() {
  return apiBase
}

export async function resolveApiBase(): Promise<string> {
  if (location.hostname !== 'wails.localhost') return ''
  if (apiBase) return apiBase
  const probes = Array.from({ length: 10 }, (_, i) => 18520 + i).map(async (port) => {
    try {
      const ctl = new AbortController()
      const timer = setTimeout(() => ctl.abort(), 1500)
      const res = await fetch(`http://127.0.0.1:${port}/api/health`, { signal: ctl.signal })
      clearTimeout(timer)
      const j = await res.json()
      if (j?.ok && j?.app === 'teacher-ledger') return `http://127.0.0.1:${port}`
    } catch {
      /* 端口不通继续下一个 */
    }
    return ''
  })
  const found = await Promise.all(probes)
  apiBase = found.find(Boolean) ?? ''
  return apiBase
}

async function req(url: string, options: RequestInit = {}) {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (auth.token) headers.Authorization = `Bearer ${auth.token}`
  const res = await fetch(apiBase + url, { headers, ...options })
  let data: any = null
  try {
    data = await res.json()
  } catch {
    /* 空响应 */
  }
  if (res.status === 401 && !url.startsWith('/api/auth/')) {
    clearSession()
    if (!location.pathname.startsWith('/login')) location.href = '/login'
  }
  if (!res.ok) throw new Error(data?.message || `请求失败（${res.status}）`)
  return data
}

export const get = (url: string) => req(url)
export const post = (url: string, body: any) => req(url, { method: 'POST', body: JSON.stringify(body) })
export const put = (url: string, body: any) => req(url, { method: 'PUT', body: JSON.stringify(body) })
export const del = (url: string) => req(url, { method: 'DELETE' })

// 需要认证的文件下载：返回原始 Response（blob 由调用方处理）
export function authDownload(url: string): Promise<Response> {
  return fetch(apiBase + url, { headers: auth.token ? { Authorization: `Bearer ${auth.token}` } : {} }).then((res) => {
    if (res.status === 401) {
      clearSession()
      location.href = '/login'
      throw new Error('登录已过期')
    }
    if (!res.ok) return res.json().then((d) => Promise.reject(new Error(d?.message || `请求失败（${res.status}）`)))
    return res
  })
}

// WebView2 内 blob <a download> 无法触发系统下载：文件字节先回传本地后端，
// 再导航到一次性下载链接，由系统下载管理器接管（与 excel.ts 导出同一链路）
export async function downloadFile(url: string, filename: string) {
  const res = await authDownload(url)
  const bytes = await res.arrayBuffer()
  const token = localStorage.getItem('ledger_token') || ''
  const saved = await fetch(`${getApiBase()}/api/export/save?name=${encodeURIComponent(filename)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/octet-stream', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: bytes,
  })
  const data = await saved.json().catch(() => null)
  if (saved.status === 401) {
    clearSession()
    location.href = '/login'
    throw new Error('登录已过期')
  }
  if (!saved.ok || !data?.url) throw new Error(data?.message || `下载失败（${saved.status}）`)
  const a = document.createElement('a')
  a.href = getApiBase() + data.url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
}

// 智能体流式对话：调用方拿到 Response 自行读取 SSE 流
export function agentStream(body: any): Promise<Response> {
  return fetch(apiBase + '/api/agent/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(auth.token ? { Authorization: `Bearer ${auth.token}` } : {}) },
    body: JSON.stringify(body),
  }).then((res) => {
    if (res.status === 401) {
      clearSession()
      location.href = '/login'
      throw new Error('登录已过期')
    }
    if (!res.ok) return res.json().then((d) => Promise.reject(new Error(d?.message || `请求失败（${res.status}）`)))
    return res
  })
}

// 智能体对话历史
export interface ConvItem {
  id: number
  title: string
  mode: string
  model: string
  created_at: string
  updated_at: string
  msg_count: number
  preview: string
}
export const fetchConversations = (): Promise<ConvItem[]> => get('/api/agent/conversations')
export const fetchConversation = (id: number): Promise<any> => get(`/api/agent/conversations/${id}`)
export const deleteConversation = (id: number) => del(`/api/agent/conversations/${id}`)

// 常用数据
export interface SubjectInfo {
  name: string
  color: string
  teacher: string
}
export const fetchClasses = (): Promise<string[]> => get('/api/classes/names')
export const fetchClassList = (): Promise<any[]> => get('/api/classes')
export const fetchSubjects = (): Promise<SubjectInfo[]> => get('/api/settings/subjects')
// 班级专属课程设置（班级未配置时服务端回退为全局科目表）
export const fetchClassSubjects = (className: string): Promise<SubjectInfo[]> =>
  get(`/api/classes/${encodeURIComponent(className)}/subjects`)
export const saveClassSubjects = (className: string, subjects: SubjectInfo[]): Promise<SubjectInfo[]> =>
  put(`/api/classes/${encodeURIComponent(className)}/subjects`, subjects)
export const fetchStudents = (className = ''): Promise<any[]> =>
  get(className ? `/api/students?class_name=${encodeURIComponent(className)}` : '/api/students')
