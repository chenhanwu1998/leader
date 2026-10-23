<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { fetchClasses, fetchClassSubjects, saveClassSubjects, get, post, put, type SubjectInfo } from '@/api'

const teacher = ref('')
const term = ref('')
const subjects = ref<SubjectInfo[]>([])
const classes = ref<string[]>([])
const subjectClass = ref('')
const resetInput = ref('')
const resetting = ref(false)

// 默认色板：新增学科时依次取色，也作为取色器的预选色
const PALETTE = ['#fef0f0', '#eef3ff', '#f4f0ff', '#effaf7', '#fef6ec', '#edf9ff', '#fdf2e8', '#f7efe2', '#eef9f0', '#fff7e0', '#fdeef5', '#f0fbff', '#eef1f8', '#f0faf5', '#f5f6fa']

onMounted(async () => {
  try {
    teacher.value = (await get('/api/settings/teacher_name')).value
    term.value = (await get('/api/settings/term')).value
    classes.value = await fetchClasses()
    if (classes.value.length) {
      subjectClass.value = classes.value[0]!
      await loadSubjects()
    }
  } catch {
    /* ignore */
  }
})

async function loadSubjects() {
  if (!subjectClass.value) return
  subjects.value = await fetchClassSubjects(subjectClass.value)
}

async function saveBasic() {
  await put('/api/settings/teacher_name', { value: teacher.value })
  await put('/api/settings/term', { value: term.value })
  ElMessage.success('基本信息已保存，刷新顶部栏后生效')
}

function addSubject() {
  subjects.value.push({ name: '', color: PALETTE[subjects.value.length % PALETTE.length]!, teacher: '' })
}

async function saveSubjects() {
  if (!subjectClass.value) return ElMessage.warning('请先在「学生花名册」中建立班级')
  const names = subjects.value.map((s) => s.name.trim()).filter(Boolean)
  if (new Set(names).size !== names.length) return ElMessage.warning('存在重名科目，请先修改')
  if (!names.length) return ElMessage.warning('至少保留一个科目')
  await saveClassSubjects(subjectClass.value, subjects.value.filter((s) => s.name.trim()))
  ElMessage.success(`「${subjectClass.value}」课程设置已保存，课程表即时生效`)
}

function backup() {
  window.open('/api/backup', '_blank')
}

async function resetAll() {
  if (resetInput.value !== '确认清空') return ElMessage.warning('请输入「确认清空」以继续')
  await ElMessageBox.confirm('将删除全部学生、成绩、请假、座位表与课程表数据，且不可恢复！建议先下载备份。', '高危操作', {
    type: 'error',
    confirmButtonText: '我已了解，继续清空',
    cancelButtonText: '取消',
  })
  resetting.value = true
  try {
    await post('/api/reset', { confirm: '确认清空' })
    ElMessage.success('已清空全部数据')
    location.reload()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    resetting.value = false
  }
}
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">系统设置</h2>
        <div class="page-sub">基础信息、科目管理与数据维护</div>
      </div>
    </div>

    <div class="grid-cards" style="grid-template-columns: repeat(auto-fit, minmax(340px, 1fr))">
      <div class="card">
        <div class="card-title">基本信息</div>
        <el-form label-width="84px" style="max-width: 360px">
          <el-form-item label="教师姓名"><el-input v-model="teacher" placeholder="显示在页面右上角" /></el-form-item>
          <el-form-item label="当前学期"><el-input v-model="term" placeholder="如：2025-2026 第一学期" /></el-form-item>
          <el-button type="primary" @click="saveBasic">保存</el-button>
        </el-form>
      </div>

      <div class="card">
        <div class="card-title">课程设置（按班级）</div>
        <div style="color: #98a0b5; font-size: 12.5px; margin-bottom: 10px">
          每个班级单独设置科目与任课教师（不同班级的任课教师可以不同）。用于课程表排课着色与教师显示；成绩录入使用全局科目表。
        </div>
        <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 12px">
          <span style="color: #5a6378; font-size: 13px; font-weight: 600">班级</span>
          <el-select v-model="subjectClass" placeholder="选择班级" style="width: 160px" @change="loadSubjects">
            <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
          </el-select>
          <span v-if="!classes.length" style="color: #aab1c5; font-size: 12px">还没有班级，请先在「学生花名册」中添加</span>
        </div>
        <template v-if="subjectClass">
          <div class="subject-head">
            <span style="width: 44px">颜色</span>
            <span style="width: 110px">科目</span>
            <span style="flex: 1">任课教师</span>
            <span style="width: 34px" />
          </div>
          <div v-for="(s, i) in subjects" :key="i" class="subject-row">
            <el-color-picker v-model="s.color" size="small" :predefine="PALETTE" />
            <el-input v-model="s.name" size="small" placeholder="科目名称" style="width: 110px" />
            <el-input v-model="s.teacher" size="small" placeholder="教师姓名（可空）" style="flex: 1" />
            <el-button link type="danger" size="small" @click="subjects.splice(i, 1)"><el-icon><Delete /></el-icon></el-button>
          </div>
          <div style="display: flex; gap: 10px; margin-top: 12px">
            <el-button size="small" plain @click="addSubject"><el-icon style="margin-right: 4px"><Plus /></el-icon>添加科目</el-button>
            <el-button type="primary" size="small" @click="saveSubjects">保存课程设置</el-button>
          </div>
        </template>
      </div>

      <div class="card">
        <div class="card-title">数据备份</div>
        <div style="color: #8a93a8; font-size: 13px; line-height: 1.8">
          所有数据保存在本地 SQLite 数据库文件中（<code>data/teacher-ledger.db</code>）。建议定期下载备份，换电脑时把备份文件放回 <code>data</code> 目录即可恢复。
        </div>
        <el-button type="primary" plain style="margin-top: 12px" @click="backup">
          <el-icon style="margin-right: 4px"><Download /></el-icon>下载数据库备份
        </el-button>
      </div>

      <div class="card" style="border-color: #f6d5d5">
        <div class="card-title" style="color: #d05a5a">危险区</div>
        <div style="color: #8a93a8; font-size: 13px; line-height: 1.8">
          清空系统中的全部数据（学生、成绩、请假、奖惩、沟通、座位表、课程表）。此操作不可撤销，请先下载备份。输入「确认清空」后执行：
        </div>
        <div style="display: flex; gap: 10px; margin-top: 12px">
          <el-input v-model="resetInput" placeholder="确认清空" style="width: 160px" />
          <el-button type="danger" :loading="resetting" :disabled="resetInput !== '确认清空'" @click="resetAll">清空全部数据</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.subject-head {
  display: flex;
  gap: 10px;
  align-items: center;
  color: #b0b7c9;
  font-size: 12px;
  margin-bottom: 6px;
  padding-right: 2px;
}

.subject-row {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-bottom: 8px;
}

code {
  background: #f0f3fb;
  padding: 1px 6px;
  border-radius: 5px;
  font-size: 12px;
  color: #4f6bed;
}
</style>
