<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { get, put, del } from '@/api'
import { auth } from '@/utils/auth'

interface AdminUser {
  username: string
  display_name: string
  avatar: string
  role: string
  created_at: string
  db_size: number
  stats: { students: number; classes: number; exams: number; scores: number; psy_profiles: number }
}

const users = ref<AdminUser[]>([])
const loading = ref(false)

const totals = computed(() =>
  users.value.reduce(
    (acc, u) => ({
      students: acc.students + u.stats.students,
      classes: acc.classes + u.stats.classes,
      scores: acc.scores + u.stats.scores,
      psy: acc.psy + u.stats.psy_profiles,
    }),
    { students: 0, classes: 0, scores: 0, psy: 0 },
  ),
)

async function load() {
  loading.value = true
  try {
    users.value = await get('/api/admin/users')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

function fmtSize(n: number) {
  if (!n) return '0 KB'
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

// 重置密码
const pwDialog = ref(false)
const pwForm = reactive({ username: '', new_password: '' })
const pwSaving = ref(false)
function openReset(u: AdminUser) {
  pwForm.username = u.username
  pwForm.new_password = ''
  pwDialog.value = true
}
async function doReset() {
  if (!pwForm.new_password || pwForm.new_password.length < 4) {
    ElMessage.warning('新密码至少 4 位')
    return
  }
  pwSaving.value = true
  try {
    await put(`/api/admin/users/${encodeURIComponent(pwForm.username)}/password`, { new_password: pwForm.new_password })
    ElMessage.success(`已重置 ${pwForm.username} 的密码`)
    pwDialog.value = false
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    pwSaving.value = false
  }
}

// 删除账号
async function removeUser(u: AdminUser) {
  try {
    await ElMessageBox.confirm(`删除账号「${u.display_name}（${u.username}）」会同时删除其全部台账数据，确定？`, '删除账号', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    await del(`/api/admin/users/${encodeURIComponent(u.username)}`)
    ElMessage.success('已删除')
    load()
  } catch (e: any) {
    ElMessage.error(e.message)
  }
}

// 全局公告
const announcement = ref('')
const annoSaving = ref(false)
async function loadAnno() {
  try {
    announcement.value = (await get('/api/auth/announcement')).value
  } catch {
    /* 可空 */
  }
}
onMounted(loadAnno)
async function saveAnno() {
  annoSaving.value = true
  try {
    await put('/api/admin/announcement', { value: announcement.value })
    ElMessage.success('公告已更新')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    annoSaving.value = false
  }
}
</script>

<template>
  <div class="admin-page">
    <div class="stat-cards">
      <el-card shadow="never" class="stat"><div class="num">{{ users.length }}</div><div class="label">注册账号</div></el-card>
      <el-card shadow="never" class="stat"><div class="num">{{ totals.students }}</div><div class="label">学生总数</div></el-card>
      <el-card shadow="never" class="stat"><div class="num">{{ totals.classes }}</div><div class="label">班级总数</div></el-card>
      <el-card shadow="never" class="stat"><div class="num">{{ totals.scores }}</div><div class="label">成绩记录</div></el-card>
      <el-card shadow="never" class="stat"><div class="num">{{ totals.psy }}</div><div class="label">心理档案</div></el-card>
    </div>

    <el-card shadow="never" class="card">
      <template #header>
        <div class="card-head">
          <span>账号列表</span>
          <el-button size="small" :loading="loading" @click="load">刷新</el-button>
        </div>
      </template>
      <el-table :data="users" v-loading="loading" stripe>
        <el-table-column label="用户" min-width="170">
          <template #default="{ row }">
            <div class="user-cell">
              <el-avatar :size="34" :src="row.avatar" />
              <div>
                <div class="dn">{{ row.display_name }}</div>
                <div class="un">{{ row.username }}</div>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="角色" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.role === 'admin' ? 'danger' : 'primary'" effect="plain">
              {{ row.role === 'admin' ? '管理员' : '用户' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="注册时间" width="150" />
        <el-table-column label="数据概览" min-width="230">
          <template #default="{ row }">
            <span class="dim">学生 {{ row.stats.students }} · 班级 {{ row.stats.classes }} · 成绩 {{ row.stats.scores }} · 心理 {{ row.stats.psy_profiles }}</span>
          </template>
        </el-table-column>
        <el-table-column label="数据量" width="90">
          <template #default="{ row }"><span class="dim">{{ fmtSize(row.db_size) }}</span></template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openReset(row)">重置密码</el-button>
            <el-button size="small" text type="danger" :disabled="row.username === auth.user?.username" @click="removeUser(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card shadow="never" class="card">
      <template #header><span>全局配置 · 登录页公告</span></template>
      <div class="anno-row">
        <el-input v-model="announcement" maxlength="500" placeholder="留空则不显示公告（展示在登录页顶部）" clearable />
        <el-button type="primary" :loading="annoSaving" @click="saveAnno">保存</el-button>
      </div>
    </el-card>

    <el-dialog v-model="pwDialog" :title="`重置密码：${pwForm.username}`" width="400px">
      <el-input v-model="pwForm.new_password" type="password" show-password placeholder="新密码（至少 4 位）" @keyup.enter="doReset" />
      <template #footer>
        <el-button @click="pwDialog = false">取消</el-button>
        <el-button type="primary" :loading="pwSaving" @click="doReset">确认重置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.admin-page {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.stat-cards {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 12px;
}

.stat {
  text-align: center;
  border: 1px solid #eef0f6;
  border-radius: 14px;
}

.num {
  font-size: 26px;
  font-weight: 700;
  color: #4f6bed;
}

.label {
  color: #7a839b;
  font-size: 12.5px;
  margin-top: 4px;
}

.card {
  border: 1px solid #eef0f6;
  border-radius: 14px;
}

.card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.user-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.dn {
  font-weight: 600;
}

.un {
  font-size: 12px;
  color: #a0a8bd;
}

.dim {
  color: #7a839b;
  font-size: 12.5px;
}

.anno-row {
  display: flex;
  gap: 10px;
}
</style>
