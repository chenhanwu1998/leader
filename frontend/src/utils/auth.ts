// 登录状态：token 存 localStorage，用户信息响应式共享
import { reactive } from 'vue'

export interface UserInfo {
  username: string
  display_name: string
  avatar: string
  role: string
  created_at?: string
}

const TOKEN_KEY = 'ledger_token'
const USER_KEY = 'ledger_user'

export const auth = reactive({
  token: localStorage.getItem(TOKEN_KEY) || '',
  user: (() => {
    try {
      return JSON.parse(localStorage.getItem(USER_KEY) || 'null') as UserInfo | null
    } catch {
      return null
    }
  })() as UserInfo | null,
})

export function setSession(token: string, user: UserInfo) {
  auth.token = token
  auth.user = user
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function setUser(user: UserInfo) {
  auth.user = user
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function clearSession() {
  auth.token = ''
  auth.user = null
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}
