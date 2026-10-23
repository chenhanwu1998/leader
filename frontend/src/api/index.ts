async function req(url: string, options: RequestInit = {}) {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  let data: any = null
  try {
    data = await res.json()
  } catch {
    /* 空响应 */
  }
  if (!res.ok) throw new Error(data?.message || `请求失败（${res.status}）`)
  return data
}

export const get = (url: string) => req(url)
export const post = (url: string, body: any) => req(url, { method: 'POST', body: JSON.stringify(body) })
export const put = (url: string, body: any) => req(url, { method: 'PUT', body: JSON.stringify(body) })
export const del = (url: string) => req(url, { method: 'DELETE' })

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
