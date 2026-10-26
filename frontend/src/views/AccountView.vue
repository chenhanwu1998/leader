<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { put } from '@/api'
import { auth, setUser, setSession } from '@/utils/auth'

const router = useRouter()

// —— 头像：读取文件 → 裁方压缩到 128px → dataURL ——
function pickAvatar() {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = 'image/*'
  input.onchange = () => {
    const file = input.files?.[0]
    if (!file) return
    if (file.size > 3 * 1024 * 1024) {
      ElMessage.warning('图片请小于 3MB')
      return
    }
    const reader = new FileReader()
    reader.onload = () => {
      const img = new Image()
      img.onload = () => {
        const side = Math.min(img.width, img.height)
        const canvas = document.createElement('canvas')
        canvas.width = canvas.height = 128
        const ctx = canvas.getContext('2d')!
        ctx.drawImage(img, (img.width - side) / 2, (img.height - side) / 2, side, side, 0, 0, 128, 128)
        saveAvatar(canvas.toDataURL('image/png'))
      }
      img.src = String(reader.result)
    }
    reader.readAsDataURL(file)
  }
  input.click()
}

const savingAvatar = ref(false)
async function saveAvatar(dataUrl: string) {
  savingAvatar.value = true
  try {
    const user = await put('/api/auth/profile', { avatar: dataUrl })
    setUser(user)
    ElMessage.success('头像已更新')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    savingAvatar.value = false
  }
}

async function resetAvatar() {
  savingAvatar.value = true
  try {
    const user = await put('/api/auth/profile', { avatar: '' })
    setUser(user)
    ElMessage.success('已恢复默认头像')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    savingAvatar.value = false
  }
}

// —— 昵称 ——
const nameForm = reactive({ display_name: auth.user?.display_name || '' })
const savingName = ref(false)
async function saveName() {
  if (!nameForm.display_name.trim()) {
    ElMessage.warning('昵称不能为空')
    return
  }
  savingName.value = true
  try {
    const user = await put('/api/auth/profile', { display_name: nameForm.display_name.trim() })
    setUser(user)
    ElMessage.success('昵称已更新')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    savingName.value = false
  }
}

// —— 修改密码 ——
const pwForm = reactive({ old_password: '', new_password: '', new_password2: '' })
const savingPw = ref(false)
async function savePassword() {
  if (!pwForm.old_password || !pwForm.new_password) {
    ElMessage.warning('请填写原密码和新密码')
    return
  }
  if (pwForm.new_password !== pwForm.new_password2) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  savingPw.value = true
  try {
    const r = await put('/api/auth/password', { old_password: pwForm.old_password, new_password: pwForm.new_password })
    if (r.token && auth.user) setSession(r.token, auth.user) // 服务端换了新会话，同步 token 避免掉线
    ElMessage.success('密码已修改')
    pwForm.old_password = pwForm.new_password = pwForm.new_password2 = ''
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    savingPw.value = false
  }
}
</script>

<template>
  <div class="account-page">
    <el-card class="card profile-card" shadow="never">
      <div class="avatar-block" @click="pickAvatar">
        <el-avatar :size="96" :src="auth.user?.avatar" class="avatar" />
        <div class="avatar-mask">更换头像</div>
      </div>
      <div class="who">
        <div class="name">{{ auth.user?.display_name }}</div>
        <div class="meta">
          <el-tag size="small" :type="auth.user?.role === 'admin' ? 'danger' : 'primary'" effect="plain">
            {{ auth.user?.role === 'admin' ? '管理员' : '普通用户' }}
          </el-tag>
          <span class="username">账号：{{ auth.user?.username }}</span>
        </div>
        <div class="created">注册时间：{{ auth.user?.created_at || '-' }}</div>
        <div class="avatar-actions">
          <el-button size="small" :loading="savingAvatar" @click="pickAvatar">上传头像</el-button>
          <el-button size="small" text type="info" @click="resetAvatar">恢复默认</el-button>
        </div>
      </div>
    </el-card>

    <el-card class="card" shadow="never">
      <template #header><span>昵称</span></template>
      <el-form label-width="80px" style="max-width: 420px">
        <el-form-item label="显示昵称">
          <el-input v-model="nameForm.display_name" maxlength="32" placeholder="给别人看的名字" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="savingName" @click="saveName">保存昵称</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="card" shadow="never">
      <template #header><span>修改密码</span></template>
      <el-form label-width="80px" style="max-width: 420px">
        <el-form-item label="原密码">
          <el-input v-model="pwForm.old_password" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="pwForm.new_password" type="password" show-password autocomplete="new-password" placeholder="至少 4 位" />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input v-model="pwForm.new_password2" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="savingPw" @click="savePassword">修改密码</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card v-if="auth.user?.role === 'admin'" class="card" shadow="never">
      <template #header><span>管理员</span></template>
      <p class="admin-tip">你是管理员，可以查看全局账号信息与配置。</p>
      <el-button type="warning" plain @click="router.push('/admin')">进入用户管理面板</el-button>
    </el-card>
  </div>
</template>

<style scoped>
.account-page {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.card {
  border: 1px solid #eef0f6;
  border-radius: 14px;
}

.profile-card {
  display: flex;
  align-items: center;
  gap: 28px;
}

.avatar-block {
  position: relative;
  cursor: pointer;
  border-radius: 50%;
  overflow: hidden;
  width: 96px;
  height: 96px;
  flex-shrink: 0;
}

.avatar-mask {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  color: #fff;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.15s;
}

.avatar-block:hover .avatar-mask {
  opacity: 1;
}

.name {
  font-size: 22px;
  font-weight: 700;
  margin-bottom: 8px;
}

.meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
}

.username,
.created {
  color: #7a839b;
  font-size: 13px;
}

.created {
  margin-bottom: 10px;
}

.admin-tip {
  color: #7a839b;
  font-size: 13px;
  margin: 0 0 12px;
}
</style>
