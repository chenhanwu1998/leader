<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { post, get } from '@/api'
import { setSession } from '@/utils/auth'

const router = useRouter()
const route = useRoute()

const mode = ref<'login' | 'register'>('login')
const loading = ref(false)
const announcement = ref('')

// 初始账号：lenhar / lenhar（梁老师）
const loginForm = reactive({ username: 'lenhar', password: 'lenhar' })
const regForm = reactive({ username: '', password: '', password2: '', display_name: '' })

onMounted(async () => {
  try {
    announcement.value = (await get('/api/auth/announcement')).value
  } catch {
    /* 公告可空 */
  }
})

async function doLogin() {
  if (!loginForm.username.trim() || !loginForm.password) {
    ElMessage.warning('请输入账号和密码')
    return
  }
  loading.value = true
  try {
    const r = await post('/api/auth/login', { username: loginForm.username.trim(), password: loginForm.password })
    setSession(r.token, r.user)
    ElMessage.success(`欢迎回来，${r.user.display_name}`)
    router.replace((route.query.r as string) || '/')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function doRegister() {
  if (!regForm.username.trim() || !regForm.password) {
    ElMessage.warning('请输入账号和密码')
    return
  }
  if (regForm.password !== regForm.password2) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }
  loading.value = true
  try {
    const r = await post('/api/auth/register', {
      username: regForm.username.trim(),
      password: regForm.password,
      display_name: regForm.display_name.trim() || undefined,
    })
    setSession(r.token, r.user)
    ElMessage.success('注册成功，已为你登录')
    router.replace('/')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-shell">
    <div class="login-card">
      <div class="brand">
        <div class="brand-logo">✎</div>
        <div>
          <div class="brand-name">教师台账系统</div>
          <div class="brand-sub">单机版 · 数据本地保存</div>
        </div>
      </div>

      <el-alert v-if="announcement" :title="announcement" type="warning" :closable="false" class="anno" />

      <el-tabs v-model="mode" stretch>
        <el-tab-pane label="登录" name="login">
          <el-form :model="loginForm" size="large" @submit.prevent="doLogin">
            <el-form-item>
              <el-input v-model="loginForm.username" placeholder="账号" autocomplete="username">
                <template #prefix><el-icon><User /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-form-item>
              <el-input
                v-model="loginForm.password"
                type="password"
                placeholder="密码"
                show-password
                autocomplete="current-password"
                @keyup.enter="doLogin"
              >
                <template #prefix><el-icon><Lock /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-button type="primary" size="large" class="submit" :loading="loading" @click="doLogin">登录</el-button>
          </el-form>
          <div class="tip">初始账号：lenhar（梁老师）· 管理员：admin</div>
        </el-tab-pane>

        <el-tab-pane label="注册新账号" name="register">
          <el-form :model="regForm" size="large" @submit.prevent="doRegister">
            <el-form-item>
              <el-input v-model="regForm.username" placeholder="账号（2-32 位字母、数字或下划线）" autocomplete="username">
                <template #prefix><el-icon><User /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-form-item>
              <el-input v-model="regForm.display_name" placeholder="昵称（可不填，默认同账号）">
                <template #prefix><el-icon><Postcard /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-form-item>
              <el-input v-model="regForm.password" type="password" placeholder="密码（至少 4 位）" show-password autocomplete="new-password">
                <template #prefix><el-icon><Lock /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-form-item>
              <el-input
                v-model="regForm.password2"
                type="password"
                placeholder="再输一次密码"
                show-password
                autocomplete="new-password"
                @keyup.enter="doRegister"
              >
                <template #prefix><el-icon><Lock /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-button type="primary" size="large" class="submit" :loading="loading" @click="doRegister">注册并登录</el-button>
          </el-form>
          <div class="tip">每个账号拥有独立的数据空间，互不影响</div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<style scoped>
.login-shell {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f6f7fb;
  background: linear-gradient(to bottom right, #eef2ff 0%, #f6f7fb 55%, #fdf3f0 100%);
}

.login-card {
  width: 400px;
  background: #fff;
  border-radius: 16px;
  padding: 30px 34px 22px;
  box-shadow: 0 12px 40px rgba(79, 107, 237, 0.13);
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 18px;
}

.brand-logo {
  width: 46px;
  height: 46px;
  border-radius: 13px;
  background: #4f6bed;
  background: linear-gradient(to bottom right, #4f6bed, #7c5bf5);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
}

.brand-name {
  font-weight: 700;
  font-size: 19px;
}

.brand-sub {
  font-size: 12px;
  color: #a0a8bd;
}

.anno {
  margin-bottom: 14px;
}

.submit {
  width: 100%;
  margin-top: 2px;
}

.tip {
  text-align: center;
  font-size: 12px;
  color: #a0a8bd;
  margin-top: 12px;
}
</style>
