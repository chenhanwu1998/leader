<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { fetchStudents } from '@/api'

const props = defineProps<{ className?: string }>()
const model = defineModel<number | null>()
const students = ref<any[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const all = await fetchStudents()
    students.value = props.className ? all.filter((s: any) => s.class_name === props.className) : all
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => props.className, load)
</script>

<template>
  <el-select v-model="model" filterable :loading="loading" placeholder="搜索学生姓名" style="width: 100%" clearable>
    <el-option v-for="s in students" :key="s.id" :value="s.id" :label="`${s.name}${s.class_name ? '（' + s.class_name + '）' : ''}`">
      <span>{{ s.name }}</span>
      <span style="float: right; color: #98a0b5; font-size: 12px">{{ s.class_name }} {{ s.student_no ? '· ' + s.student_no : '' }}</span>
    </el-option>
  </el-select>
</template>
