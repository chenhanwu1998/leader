<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { get, post } from '@/api'
import { auth, clearSession } from '@/utils/auth'

const route = useRoute()
const router = useRouter()
const isLoginPage = computed(() => route.name === 'login')
const title = computed(() => (route.meta.title as string) || '')

const term = ref('')

async function loadProfile() {
  try {
    term.value = (await get('/api/settings/term')).value
  } catch {
    /* 首次启动为空 */
  }
}
onMounted(loadProfile)

const menus = computed(() => {
  const list = [
    { path: '/', title: '总览', icon: 'Odometer' },
    { path: '/classes', title: '班级管理', icon: 'School' },
    { path: '/students', title: '学生花名册', icon: 'User' },
    { path: '/scores', title: '成绩管理', icon: 'TrendCharts' },
    { path: '/leaves', title: '请假管理', icon: 'Calendar' },
    { path: '/rewards', title: '奖惩记录', icon: 'Medal' },
    { path: '/contacts', title: '家校联系', icon: 'ChatDotSquare' },
    { path: '/seats', title: '座位表', icon: 'Grid' },
    { path: '/schedule', title: '课程表', icon: 'Clock' },
    { path: '/psy', title: '心理健康', icon: 'FirstAidKit' },
    { path: '/agent', title: '智能助手', icon: 'MagicStick' },
    { path: '/settings', title: '系统设置', icon: 'Setting' },
  ]
  if (auth.user?.role === 'admin') list.push({ path: '/admin', title: '用户管理', icon: 'UserFilled' })
  return list
})

async function onUserCommand(cmd: string) {
  if (cmd === 'account') {
    router.push('/account')
  } else if (cmd === 'account-settings') {
    router.push('/settings')
  } else if (cmd === 'logout') {
    try {
      await post('/api/auth/logout', {})
    } catch {
      /* 会话已不存在也照常登出 */
    }
    clearSession()
    router.replace('/login')
  }
}
</script>

<template>
  <router-view v-if="isLoginPage" />

  <el-container v-else class="app-shell">
    <el-aside width="216px" class="aside">
      <div class="brand">
        <div class="brand-logo">✎</div>
        <div>
          <div class="brand-name">教师台账</div>
          <div class="brand-sub">Teacher Ledger</div>
        </div>
      </div>
      <el-menu :default-active="route.path" router class="menu" :collapse="false">
        <el-menu-item v-for="m in menus" :key="m.path" :index="m.path" class="menu-item">
          <el-icon><component :is="m.icon" /></el-icon>
          <span>{{ m.title }}</span>
        </el-menu-item>
      </el-menu>
      <div class="aside-footer">单机版 · 数据本地保存</div>
    </el-aside>

    <el-container>
      <el-header height="60px" class="topbar">
        <div class="topbar-title">{{ title }}</div>
        <div class="topbar-right">
          <el-tag v-if="term" effect="plain" round type="primary">{{ term }}</el-tag>
          <el-dropdown trigger="click" @command="onUserCommand">
            <div class="user-trigger">
              <el-avatar :size="30" :src="auth.user?.avatar" class="avatar" />
              <span class="nickname">{{ auth.user?.display_name || '未登录' }}</span>
              <el-icon class="caret"><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="account"><el-icon><Postcard /></el-icon>个人中心</el-dropdown-item>
                <el-dropdown-item command="account-settings"><el-icon><Setting /></el-icon>系统设置</el-dropdown-item>
                <el-dropdown-item command="logout" divided><el-icon><SwitchButton /></el-icon>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main class="main">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>

  </el-container>
</template>

<style scoped>
.app-shell {
  height: 100%;
}

.aside {
  background: #fff;
  border-right: 1px solid #eef0f6;
  display: flex;
  flex-direction: column;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 18px 16px 14px;
}

.brand-logo {
  width: 38px;
  height: 38px;
  border-radius: 11px;
  background: #4f6bed;
  background: linear-gradient(to bottom right, #4f6bed, #7c5bf5);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
}

.brand-name {
  font-weight: 700;
  font-size: 16px;
}

.brand-sub {
  font-size: 11px;
  color: #a0a8bd;
  letter-spacing: 0.4px;
}

.menu {
  border-right: none;
  flex: 1;
  padding: 6px 10px;
  overflow-y: auto;
}

.menu-item {
  border-radius: 10px;
  margin-bottom: 4px;
  height: 44px;
}

.menu-item.is-active {
  background: #eef2ff;
  font-weight: 600;
}

.aside-footer {
  padding: 14px 18px;
  font-size: 11.5px;
  color: #b0b7c9;
  border-top: 1px solid #f2f4f9;
}

.topbar {
  background: rgba(255, 255, 255, 0.85);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid #eef0f6;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.topbar-title {
  font-size: 16px;
  font-weight: 650;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
  color: #5a6378;
  font-size: 13px;
}

.user-trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 5px 10px;
  border-radius: 999px;
  transition: background 0.15s ease;
  outline: none;
}

.user-trigger:hover {
  background: #eef2ff;
}

.nickname {
  font-weight: 600;
  color: #3a4257;
  font-size: 13px;
}

.caret {
  color: #a0a8bd;
  font-size: 12px;
}

.main {
  padding: 0;
  overflow-y: auto;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
