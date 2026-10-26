import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { auth } from '@/utils/auth'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { title: '登录', public: true } },
  { path: '/', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { title: '总览' } },
  { path: '/classes', name: 'classes', component: () => import('@/views/ClassesView.vue'), meta: { title: '班级管理' } },
  { path: '/students', name: 'students', component: () => import('@/views/StudentsView.vue'), meta: { title: '学生花名册' } },
  { path: '/scores', name: 'scores', component: () => import('@/views/ScoresView.vue'), meta: { title: '成绩管理' } },
  { path: '/leaves', name: 'leaves', component: () => import('@/views/LeavesView.vue'), meta: { title: '请假管理' } },
  { path: '/rewards', name: 'rewards', component: () => import('@/views/RewardsView.vue'), meta: { title: '奖惩记录' } },
  { path: '/contacts', name: 'contacts', component: () => import('@/views/ContactsView.vue'), meta: { title: '家校联系' } },
  { path: '/seats', name: 'seats', component: () => import('@/views/SeatsView.vue'), meta: { title: '座位表' } },
  { path: '/schedule', name: 'schedule', component: () => import('@/views/ScheduleView.vue'), meta: { title: '课程表' } },
  { path: '/psy', name: 'psy', component: () => import('@/views/PsyView.vue'), meta: { title: '心理健康' } },
  { path: '/agent', name: 'agent', component: () => import('@/views/AgentView.vue'), meta: { title: '智能助手' } },
  { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue'), meta: { title: '系统设置' } },
  { path: '/account', name: 'account', component: () => import('@/views/AccountView.vue'), meta: { title: '个人中心' } },
  { path: '/admin', name: 'admin', component: () => import('@/views/AdminView.vue'), meta: { title: '用户管理', admin: true } },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  if (to.meta.public) {
    // 已登录时跳过登录页
    if (auth.token) return { path: '/' }
    return true
  }
  if (!auth.token) return { path: '/login', query: to.fullPath !== '/' ? { r: to.fullPath } : {} }
  if (to.meta.admin && auth.user?.role !== 'admin') return { path: '/' }
  return true
})

router.afterEach((to) => {
  document.title = `${to.meta.title ?? ''} · 教师台账系统`
})

export default router
