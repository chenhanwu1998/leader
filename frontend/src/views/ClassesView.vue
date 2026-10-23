<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { del, fetchClassList, post, put } from '@/api'
import { exportExcel } from '@/utils/excel'

const loading = ref(false)
const list = ref<any[]>([])
const dialogVisible = ref(false)
const editing = ref<any>(null)
const saving = ref(false)
const form = ref<any>({})
const blank = () => ({ name: '', grade: '', head_teacher: '', classroom: '', remark: '' })

const GRADES = ['一年级', '二年级', '三年级', '四年级', '五年级', '六年级', '初一', '初二', '初三', '高一', '高二', '高三']

const totalStudents = computed(() => list.value.reduce((a, c) => a + (c.student_count ?? 0), 0))
const avgStudents = computed(() => (list.value.length ? Math.round((totalStudents.value / list.value.length) * 10) / 10 : 0))

async function load() {
  loading.value = true
  try {
    list.value = await fetchClassList()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

function openAdd() {
  editing.value = null
  form.value = blank()
  dialogVisible.value = true
}

function openEdit(row: any) {
  editing.value = row
  form.value = { name: row.name, grade: row.grade, head_teacher: row.head_teacher, classroom: row.classroom, remark: row.remark }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.name?.trim()) {
    ElMessage.warning('请填写班级名称')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      const res = await put(`/api/classes/${editing.value.id}`, form.value)
      ElMessage.success(res.renamed ? '已更新班级，相关学生、座位表、课程表的班级名已同步修改' : '已更新班级信息')
    } else {
      await post('/api/classes', form.value)
      ElMessage.success('已创建班级')
    }
    dialogVisible.value = false
    load()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function remove(row: any) {
  await ElMessageBox.confirm(
    row.student_count > 0
      ? `班级「${row.name}」下还有 ${row.student_count} 名学生，需要先将学生调班或删除后才能删除班级。`
      : `确定删除班级「${row.name}」？其座位表与课程表会一并删除。`,
    '删除确认',
    { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消', confirmButtonClass: row.student_count > 0 ? 'el-button--danger is-disabled' : '' },
  )
  if (row.student_count > 0) return
  try {
    await del(`/api/classes/${row.id}`)
    ElMessage.success('已删除班级')
    load()
  } catch (e: any) {
    ElMessage.error(e.message)
  }
}

const exportRows = computed(() =>
  list.value.map((c) => ({ 班级名称: c.name, 年级: c.grade, 班主任: c.head_teacher, 教室: c.classroom, 学生人数: c.student_count, 备注: c.remark })),
)
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">班级管理</h2>
        <div class="page-sub">维护班级信息；学生的班级、座位表、课程表都与这里的班级关联</div>
      </div>
    </div>

    <div class="stat-cards" style="margin-bottom: 16px">
      <div class="stat-card">
        <div class="stat-icon" style="background: #eef2ff">🏫</div>
        <div><div class="stat-num">{{ list.length }}</div><div class="stat-label">班级总数</div></div>
      </div>
      <div class="stat-card">
        <div class="stat-icon" style="background: #effaf7">🎓</div>
        <div><div class="stat-num">{{ totalStudents }}</div><div class="stat-label">在读学生总数</div></div>
      </div>
      <div class="stat-card">
        <div class="stat-icon" style="background: #fef6ec">📊</div>
        <div><div class="stat-num">{{ avgStudents }}</div><div class="stat-label">平均每班人数</div></div>
      </div>
    </div>

    <div class="card">
      <div class="toolbar" style="margin-bottom: 14px">
        <div style="color: #8a93a8; font-size: 13px">💡 修改班级名称会自动同步学生、座位表与课程表上的班级名</div>
        <div style="flex: 1" />
        <el-button @click="exportExcel(exportRows, '班级列表')"><el-icon style="margin-right: 4px"><Download /></el-icon>导出</el-button>
        <el-button type="primary" @click="openAdd"><el-icon style="margin-right: 4px"><Plus /></el-icon>新建班级</el-button>
      </div>

      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="name" label="班级名称" width="140">
          <template #default="{ row }"><b style="color: #4f6bed">{{ row.name }}</b></template>
        </el-table-column>
        <el-table-column prop="grade" label="年级" width="100" />
        <el-table-column prop="head_teacher" label="班主任" width="110" />
        <el-table-column prop="classroom" label="教室" width="150" show-overflow-tooltip />
        <el-table-column prop="student_count" label="学生人数" width="100" sortable>
          <template #default="{ row }">
            <el-tag size="small" :type="row.student_count > 0 ? 'primary' : 'info'" effect="light">{{ row.student_count }} 人</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="140" show-overflow-tooltip />
        <el-table-column label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty><div class="empty-tip">暂无班级，点击右上角「新建班级」创建；或通过学生花名册批量导入自动建班</div></template>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑班级' : '新建班级'" width="480px">
      <el-form :model="form" label-width="84px">
        <el-form-item label="班级名称" required>
          <el-input v-model="form.name" placeholder="如：三年级1班" />
        </el-form-item>
        <el-form-item label="年级">
          <el-select v-model="form.grade" filterable allow-create default-first-option clearable placeholder="选择或输入年级" style="width: 100%">
            <el-option v-for="g in GRADES" :key="g" :value="g" :label="g" />
          </el-select>
        </el-form-item>
        <el-form-item label="班主任"><el-input v-model="form.head_teacher" placeholder="班主任姓名" /></el-form-item>
        <el-form-item label="教室"><el-input v-model="form.classroom" placeholder="如：教学楼2层201" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
