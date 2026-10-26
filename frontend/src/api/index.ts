import { auth, clearSession } from '@/utils/auth'

async function req(url: string, options: RequestInit = {}) {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (auth.token) headers.Authorization = `Bearer ${auth.token}`
  const res = await fetch(url, { headers, ...options })
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
  return fetch(url, { headers: auth.token ? { Authorization: `Bearer ${auth.token}` } : {} }).then((res) => {
    if (res.status === 401) {
      clearSession()
      location.href = '/login'
      throw new Error('登录已过期')
    }
    if (!res.ok) return res.json().then((d) => Promise.reject(new Error(d?.message || `请求失败（${res.status}）`)))
    return res
  })
}

// 智能体流式对话：调用方拿到 Response 自行读取 SSE 流
export function agentStream(body: any): Promise<Response> {
  return fetch('/api/agent/chat', {
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
