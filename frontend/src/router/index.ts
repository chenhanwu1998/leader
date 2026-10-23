import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  { path: '/', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { title: '总览' } },
  { path: '/classes', name: 'classes', component: () => import('@/views/ClassesView.vue'), meta: { title: '班级管理' } },
  { path: '/students', name: 'students', component: () => import('@/views/StudentsView.vue'), meta: { title: '学生花名册' } },
  { path: '/scores', name: 'scores', component: () => import('@/views/ScoresView.vue'), meta: { title: '成绩管理' } },
  { path: '/leaves', name: 'leaves', component: () => import('@/views/LeavesView.vue'), meta: { title: '请假管理' } },
  { path: '/rewards', name: 'rewards', component: () => import('@/views/RewardsView.vue'), meta: { title: '奖惩记录' } },
  { path: '/contacts', name: 'contacts', component: () => import('@/views/ContactsView.vue'), meta: { title: '家校联系' } },
  { path: '/seats', name: 'seats', component: () => import('@/views/SeatsView.vue'), meta: { title: '座位表' } },
  { path: '/schedule', name: 'schedule', component: () => import('@/views/ScheduleView.vue'), meta: { title: '课程表' } },
  { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue'), meta: { title: '系统设置' } },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.afterEach((to) => {
  document.title = `${to.meta.title ?? ''} · 教师台账系统`
})

export default router
