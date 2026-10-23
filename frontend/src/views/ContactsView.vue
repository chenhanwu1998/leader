<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { del, fetchClasses, get, post, put } from '@/api'
import { exportCsv, exportExcel } from '@/utils/excel'
import ImportDialog from '@/components/ImportDialog.vue'
import StudentSelect from '@/components/StudentSelect.vue'

const loading = ref(false)
const list = ref<any[]>([])
const classes = ref<string[]>([])
const filter = reactive({ class_name: '', keyword: '', way: '', dateRange: null as null | [string, string] })
const formClass = ref('') // 弹窗内用于缩小学生检索范围的班级
const page = ref(1)
const pageSize = ref(15)
const dialogVisible = ref(false)
const editing = ref<any>(null)
const saving = ref(false)
const importVisible = ref(false)

const form = ref<any>({})
const blank = () => ({ student_id: null, contact_date: new Date().toISOString().slice(0, 10), way: '电话', target: '家长', content: '', feedback: '', remark: '' })

const importColumns = [
  { key: 'student_name', label: '姓名', required: true },
  { key: 'class_name', label: '班级' },
  { key: 'contact_date', label: '日期' },
  { key: 'way', label: '方式' },
  { key: 'target', label: '对象' },
  { key: 'content', label: '沟通内容' },
  { key: 'feedback', label: '家长反馈' },
  { key: 'remark', label: '备注' },
]
const importSample = [{ 姓名: '张三', 班级: '一年级1班', 日期: '2026-10-18', 方式: '电话', 对象: '家长', 沟通内容: '反馈近期学习状态', 家长反馈: '会加强督促', 备注: '' }]

const paged = computed(() => list.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
const monthCount = computed(() => {
  const m = new Date().toISOString().slice(0, 7)
  return list.value.filter((r) => (r.contact_date || '').startsWith(m)).length
})

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams()
    if (filter.class_name) params.set('class_name', filter.class_name)
    if (filter.keyword) params.set('keyword', filter.keyword)
    if (filter.way) params.set('way', filter.way)
    if (filter.dateRange?.[0]) params.set('date_from', filter.dateRange[0])
    if (filter.dateRange?.[1]) params.set('date_to', filter.dateRange[1])
    list.value = await get(`/api/contacts?${params}`)
    page.value = 1
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  classes.value = await fetchClasses()
  load()
})

function openAdd() {
  editing.value = null
  form.value = blank()
  formClass.value = ''
  dialogVisible.value = true
}
function openEdit(row: any) {
  editing.value = row
  form.value = { ...row }
  formClass.value = row.class_name ?? ''
  dialogVisible.value = true
}

async function save() {
  if (!form.value.student_id) return ElMessage.warning('请选择学生')
  saving.value = true
  try {
    if (editing.value) await put(`/api/contacts/${editing.value.id}`, form.value)
    else await post('/api/contacts', form.value)
    ElMessage.success('已保存沟通记录')
    dialogVisible.value = false
    load()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function remove(row: any) {
  await ElMessageBox.confirm('确定删除这条沟通记录？', '删除确认', { type: 'warning', confirmButtonText: '删除' })
  await del(`/api/contacts/${row.id}`)
  ElMessage.success('已删除')
  load()
}

async function onImport(records: any[]) {
  const res = await post('/api/contacts/batch', { records })
  if (res.fail) ElMessage.warning(`导入完成：成功 ${res.ok} 条，失败 ${res.fail} 条。${res.errors?.[0] ?? ''}`)
  else ElMessage.success(`成功导入 ${res.ok} 条沟通记录`)
  load()
}

const exportRows = computed(() =>
  list.value.map((r) => ({ 姓名: r.student_name, 班级: r.class_name, 日期: r.contact_date, 方式: r.way, 对象: r.target, 沟通内容: r.content, 家长反馈: r.feedback, 备注: r.remark })),
)
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">家校联系记录</h2>
        <div class="page-sub">记录与家长的电话、微信、家访等沟通内容与反馈</div>
      </div>
    </div>

    <div class="stat-cards" style="margin-bottom: 16px">
      <div class="stat-card">
        <div class="stat-icon" style="background: #eef2ff">💬</div>
        <div><div class="stat-num">{{ list.length }}</div><div class="stat-label">累计沟通记录</div></div>
      </div>
      <div class="stat-card">
        <div class="stat-icon" style="background: #effaf7">📆</div>
        <div><div class="stat-num">{{ monthCount }}</div><div class="stat-label">本月沟通次数</div></div>
      </div>
      <div class="stat-card">
        <div class="stat-icon" style="background: #f4f0ff">🧑‍🤝‍🧑</div>
        <div><div class="stat-num">{{ new Set(list.map((r) => r.student_id)).size }}</div><div class="stat-label">覆盖学生数</div></div>
      </div>
    </div>

    <div class="card">
      <div class="toolbar" style="margin-bottom: 14px">
        <el-input v-model="filter.keyword" placeholder="搜索姓名 / 内容" clearable style="width: 160px" @keyup.enter="load" @clear="load" />
        <el-select v-model="filter.class_name" placeholder="班级" clearable filterable style="width: 130px" @change="load">
          <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
        </el-select>
        <el-select v-model="filter.way" placeholder="方式" clearable style="width: 96px" @change="load">
          <el-option v-for="w in ['电话', '微信', '面谈', '家访', '其他']" :key="w" :value="w" :label="w" />
        </el-select>
        <el-date-picker v-model="filter.dateRange" type="daterange" value-format="YYYY-MM-DD" range-separator="至"
          start-placeholder="开始日期" end-placeholder="结束日期" style="width: 240px" @change="load" />
        <el-button type="primary" plain @click="load">查询</el-button>
        <div style="flex: 1" />
        <el-button @click="importVisible = true"><el-icon style="margin-right: 4px"><Upload /></el-icon>导入</el-button>
        <el-dropdown>
          <el-button><el-icon style="margin-right: 4px"><Download /></el-icon>导出<el-icon style="margin-left: 4px"><ArrowDown /></el-icon></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item @click="exportExcel(exportRows, '家校联系记录')">导出 Excel</el-dropdown-item>
              <el-dropdown-item @click="exportCsv(exportRows, '家校联系记录')">导出 CSV</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button type="primary" @click="openAdd"><el-icon style="margin-right: 4px"><Plus /></el-icon>添加记录</el-button>
      </div>

      <el-table :data="paged" v-loading="loading" stripe>
        <el-table-column prop="contact_date" label="日期" width="108" sortable />
        <el-table-column prop="student_name" label="学生" width="92">
          <template #default="{ row }"><b>{{ row.student_name }}</b></template>
        </el-table-column>
        <el-table-column prop="class_name" label="班级" width="112" />
        <el-table-column prop="way" label="方式" width="80">
          <template #default="{ row }"><el-tag size="small" effect="light">{{ row.way }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="target" label="对象" width="70" />
        <el-table-column prop="content" label="沟通内容" min-width="190" show-overflow-tooltip />
        <el-table-column prop="feedback" label="家长反馈" min-width="150" show-overflow-tooltip />
        <el-table-column label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty><div class="empty-tip">暂无沟通记录</div></template>
      </el-table>

      <div style="display: flex; justify-content: flex-end; margin-top: 12px">
        <el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="list.length" :page-sizes="[15, 30, 50]" layout="total, sizes, prev, pager, next" background />
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑沟通记录' : '添加沟通记录'" width="560px">
      <el-form :model="form" label-width="84px">
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="班级">
              <el-select v-model="formClass" placeholder="先选班级便于检索" clearable filterable style="width: 100%" @change="form.student_id = null">
                <el-option v-for="c in classes" :key="c" :value="c" :label="c" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="学生" required label-width="64px">
              <StudentSelect v-model="form.student_id" :class-name="formClass" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="14">
          <el-col :span="12"><el-form-item label="日期"><el-date-picker v-model="form.contact_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" /></el-form-item></el-col>
          <el-col :span="12">
            <el-form-item label="方式">
              <el-select v-model="form.way" style="width: 100%">
                <el-option v-for="w in ['电话', '微信', '面谈', '家访', '其他']" :key="w" :value="w" :label="w" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="沟通对象">
          <el-radio-group v-model="form.target">
            <el-radio v-for="t in ['家长', '学生', '其他']" :key="t" :value="t">{{ t }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="沟通内容"><el-input v-model="form.content" type="textarea" :rows="3" placeholder="沟通的事项、学生近况等" /></el-form-item>
        <el-form-item label="家长反馈"><el-input v-model="form.feedback" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <ImportDialog v-model="importVisible" title="家校联系记录" :columns="importColumns" :sample="importSample"
      tip="姓名需与花名册一致；方式支持「电话 / 微信 / 面谈 / 家访 / 其他」" @confirm="onImport" />
  </div>
</template>
