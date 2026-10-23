<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { downloadTemplate, mapHeaders, readExcel, type RowData } from '@/utils/excel'

export interface ImportColumn {
  key: string
  label: string
  required?: boolean
  synonyms?: string[]
}

const props = defineProps<{
  title: string
  columns: ImportColumn[]
  sample: RowData[]
  tip?: string
}>()

const emit = defineEmits<{ confirm: [records: RowData[]] }>()
const visible = defineModel<boolean>({ required: true })

const rawRows = ref<RowData[]>([])
const fileName = ref('')
const parsing = ref(false)

const records = computed(() => mapHeaders(rawRows.value, props.columns))
const preview = computed(() => records.value.slice(0, 8))
const missingRequired = computed(() => {
  if (!records.value.length) return []
  return props.columns.filter((c) => c.required && records.value[0] && records.value[0][c.key] === '').map((c) => c.label)
})

async function onFileChange(file: any) {
  parsing.value = true
  try {
    fileName.value = file.name
    const rows = await readExcel(file.raw as File)
    if (!rows.length) {
      ElMessage.warning('文件中没有读取到数据，请检查表头是否与模板一致')
      rawRows.value = []
      return
    }
    rawRows.value = rows
    ElMessage.success(`成功读取 ${rows.length} 行数据`)
  } catch (e: any) {
    ElMessage.error(`文件解析失败：${e.message}`)
    rawRows.value = []
  } finally {
    parsing.value = false
  }
}

function confirm() {
  if (!records.value.length) return
  emit('confirm', records.value)
  reset()
  visible.value = false
}

function reset() {
  rawRows.value = []
  fileName.value = ''
}

function close() {
  reset()
  visible.value = false
}
</script>

<template>
  <el-dialog :model-value="visible" :title="`导入${title}`" width="720px" @close="close" append-to-body>
    <div class="import-steps">
      <div class="step">
        <div class="step-no">1</div>
        <div>
          <div class="step-title">下载模板</div>
          <div class="step-desc">按模板填写数据，表头需与模板一致（支持 .xlsx / .csv）</div>
        </div>
        <el-button size="small" plain type="primary" @click="downloadTemplate(sample, `${title}导入模板`)">
          下载模板
        </el-button>
      </div>
      <div class="step">
        <div class="step-no">2</div>
        <div style="flex: 1">
          <div class="step-title">选择文件</div>
          <el-upload :auto-upload="false" :show-file-list="false" accept=".xlsx,.xls,.csv" :on-change="onFileChange" drag>
            <div class="upload-inner">
              <el-icon size="26" color="#8a93a8"><UploadFilled /></el-icon>
              <span>{{ fileName || '点击或拖拽文件到此处上传' }}</span>
            </div>
          </el-upload>
        </div>
      </div>
    </div>

    <el-alert v-if="missingRequired.length" type="error" :closable="false" style="margin-bottom: 10px"
      :title="`缺少必填列：${missingRequired.join('、')}，请检查文件表头`" />
    <el-alert v-else-if="tip" type="info" :closable="false" style="margin-bottom: 10px" :title="tip" />

    <template v-if="records.length">
      <div class="preview-head">
        数据预览（共 {{ records.length }} 行，预览前 {{ preview.length }} 行）
      </div>
      <el-table :data="preview" size="small" border max-height="260">
        <el-table-column v-for="c in columns" :key="c.key" :prop="c.key" :label="c.label" min-width="80" show-overflow-tooltip />
      </el-table>
    </template>

    <template #footer>
      <el-button @click="close">取消</el-button>
      <el-button type="primary" :disabled="!records.length || !!missingRequired.length" :loading="parsing" @click="confirm">
        确认导入 {{ records.length ? `${records.length} 条` : '' }}
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.import-steps {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 14px;
}

.step {
  display: flex;
  align-items: center;
  gap: 12px;
}

.step-no {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: #eef2ff;
  color: #4f6bed;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.step-title {
  font-weight: 600;
  font-size: 13.5px;
}

.step-desc {
  color: #98a0b5;
  font-size: 12px;
}

.step > :nth-child(2) {
  flex: 1;
}

.upload-inner {
  padding: 10px 0;
  color: #8a93a8;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.preview-head {
  font-size: 13px;
  color: #5a6378;
  margin-bottom: 8px;
}
</style>
