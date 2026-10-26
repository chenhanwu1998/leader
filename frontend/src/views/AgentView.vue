<script setup lang="ts">
import { nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { get, agentStream } from '@/api'
import { auth } from '@/utils/auth'
import { renderMarkdown } from '@/utils/markdown'

interface Msg {
  role: 'user' | 'assistant'
  content: string
  sources?: { n: number; title: string }[]
}

const meta = ref<{ models: { id: string; label: string }[]; default_model: string; ready: boolean; knowledge_base: boolean } | null>(null)
const model = ref('kimi-k3')
const psych = ref(false)
const input = ref('')
const sending = ref(false)
const listEl = ref<HTMLElement>()

const messages = reactive<Msg[]>([])

const QUICK_PROMPTS = [
  { psych: false, text: '帮我写一段期中考试后家长会的开场发言' },
  { psych: false, text: '班里几个学生最近上课总是睡觉，可能有哪些原因？怎么排查？' },
  { psych: true, text: '学生 PHQ-9 第9题选了2分，我该怎么处理？' },
  { psych: true, text: '怎么和疑似有抑郁倾向的学生进行第一次谈话？' },
]

onMounted(async () => {
  try {
    meta.value = await get('/api/agent/meta')
    model.value = meta.value.default_model
  } catch {
    /* meta 失败也能发，后端有兜底 */
  }
})

function scrollBottom() {
  nextTick(() => {
    if (listEl.value) listEl.value.scrollTop = listEl.value.scrollHeight
  })
}

async function send(text?: string) {
  const content = (text ?? input.value).trim()
  if (!content || sending.value) return
  input.value = ''
  messages.push({ role: 'user', content })
  const assistantMsg = reactive<Msg>({ role: 'assistant', content: '' })
  messages.push(assistantMsg)
  scrollBottom()
  sending.value = true
  try {
    const res = await agentStream({
      model: model.value,
      psych: psych.value,
      messages: messages.filter((m) => m !== assistantMsg).map((m) => ({ role: m.role, content: m.content })),
    })
    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    let event = ''
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      let idx
      while ((idx = buf.indexOf('\n\n')) >= 0) {
        const block = buf.slice(0, idx)
        buf = buf.slice(idx + 2)
        event = ''
        for (const line of block.split('\n')) {
          if (line.startsWith('event:')) {
            event = line.slice(6).trim()
          } else if (line.startsWith('data:')) {
            const payload = line.slice(5).trim()
            if (event === 'sources') {
              try {
                assistantMsg.sources = JSON.parse(payload)
              } catch {
                /* 忽略 */
              }
            } else if (payload === '[DONE]') {
              /* 流结束 */
            } else {
              try {
                const j = JSON.parse(payload)
                const delta = j.choices?.[0]?.delta?.content
                if (delta) assistantMsg.content += delta
              } catch {
                /* 非 JSON 片段忽略 */
              }
            }
          }
        }
        scrollBottom()
      }
    }
    if (!assistantMsg.content) assistantMsg.content = '（没有收到回复内容）'
  } catch (e: any) {
    assistantMsg.content = `⚠️ ${e.message}`
    ElMessage.error(e.message)
  } finally {
    sending.value = false
    scrollBottom()
  }
}

function clearChat() {
  messages.splice(0)
}

function usePrompt(p: { psych: boolean; text: string }) {
  psych.value = p.psych
  send(p.text)
}
</script>

<template>
  <div class="agent-page">
    <div class="chat-panel">
      <div class="panel-head">
        <div class="head-left">
          <el-select v-model="model" size="small" style="width: 220px">
            <el-option v-for="m in meta?.models ?? []" :key="m.id" :value="m.id" :label="m.label" />
          </el-select>
          <el-switch v-model="psych" active-text="心理模式" inactive-text="通用模式" class="mode-switch" />
          <el-tooltip v-if="psych" content="心理模式会先检索校园心理健康知识库，再结合资料作答" placement="top">
            <el-icon class="kb" :class="{ on: meta?.knowledge_base }"><Collection /></el-icon>
          </el-tooltip>
        </div>
        <el-button size="small" text type="info" @click="clearChat">清空对话</el-button>
      </div>

      <div ref="listEl" class="msg-list">
        <div v-if="!messages.length" class="empty">
          <div class="empty-logo">🤖</div>
          <p>你好，{{ auth.user?.display_name }}。我是你的智能助手。</p>
          <p class="dim">通用模式处理班级事务；打开「心理模式」后会优先检索校园心理健康知识库作答。</p>
          <div class="prompts">
            <el-button v-for="p in QUICK_PROMPTS" :key="p.text" size="small" round plain @click="usePrompt(p)">{{ p.text }}</el-button>
          </div>
        </div>

        <div v-for="(m, i) in messages" :key="i" class="msg" :class="m.role">
          <div class="bubble">
            <div v-if="m.role === 'assistant'" class="text md">
              <span v-html="renderMarkdown(m.content)"></span><span v-if="sending && i === messages.length - 1" class="caret">▌</span>
            </div>
            <div v-else class="text">{{ m.content }}</div>
            <div v-if="m.sources?.length" class="sources">
              <span class="src-label">参考知识库：</span>
              <el-tag v-for="s in [...new Set(m.sources.map((x) => x.title))]" :key="s" size="small" effect="plain" class="src-tag">{{ s }}</el-tag>
            </div>
          </div>
        </div>
      </div>

      <div class="input-row">
        <el-input
          v-model="input"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 6 }"
          resize="none"
          :placeholder="psych ? '描述学生情况或心理相关问题…（Enter 发送）' : '想问点什么？（Enter 发送）'"
          @keydown.enter.exact.prevent="send()"
        />
        <el-button type="primary" class="send-btn" :loading="sending" :disabled="!input.trim()" @click="send()">发送</el-button>
      </div>
      <div class="disclaimer">智能体回答仅供参考；涉及学生安全与心理健康危机时，请立即线下介入并联系专业人员。</div>
    </div>
  </div>
</template>

<style scoped>
.agent-page {
  height: 100%;
  padding: 16px 20px;
  box-sizing: border-box;
  display: flex;
}

.chat-panel {
  flex: 1;
  background: #fff;
  border: 1px solid #eef0f6;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  border-bottom: 1px solid #f2f4f9;
}

.head-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.kb {
  color: #c3c9d9;
  font-size: 18px;
}

.kb.on {
  color: #4f6bed;
}

.msg-list {
  flex: 1;
  overflow-y: auto;
  padding: 18px 16px;
}

.empty {
  text-align: center;
  color: #5a6378;
  margin-top: 8vh;
}

.empty-logo {
  font-size: 44px;
}

.dim {
  color: #a0a8bd;
  font-size: 13px;
}

.prompts {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: center;
  margin-top: 16px;
}

.prompts .el-button + .el-button {
  margin-left: 0;
}

.msg {
  display: flex;
  margin-bottom: 14px;
}

.msg.user {
  justify-content: flex-end;
}

.bubble {
  max-width: 78%;
  padding: 10px 14px;
  border-radius: 12px;
  background: #f4f6fd;
  color: #2f3646;
}

.msg.user .bubble {
  background: #4f6bed;
  background: linear-gradient(to bottom right, #4f6bed, #6e8aft);
  color: #fff;
}

.text {
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.65;
  font-size: 14px;
}

.text.md {
  white-space: normal;
}

.text.md :deep(.md-h) {
  font-weight: 700;
  color: #22304a;
  margin: 10px 0 4px;
  line-height: 1.4;
}

.text.md :deep(.md-h1) {
  font-size: 17px;
}

.text.md :deep(.md-h2) {
  font-size: 16px;
}

.text.md :deep(.md-h3),
.text.md :deep(.md-h4),
.text.md :deep(.md-h5),
.text.md :deep(.md-h6) {
  font-size: 14.5px;
}

.text.md :deep(.md-ul),
.text.md :deep(.md-ol) {
  margin: 4px 0;
  padding-left: 20px;
}

.text.md :deep(li) {
  margin: 2px 0;
}

.text.md :deep(.md-inline) {
  background: #eceffe;
  color: #3a54c4;
  border-radius: 4px;
  padding: 0 4px;
  font-size: 13px;
}

.text.md :deep(.md-code) {
  background: #22293a;
  color: #dbe4ff;
  border-radius: 8px;
  padding: 10px 12px;
  font-size: 12.5px;
  overflow-x: auto;
  margin: 6px 0;
  white-space: pre;
}

.text.md :deep(strong) {
  color: #1f2c47;
}

.caret {
  animation: blink 0.9s steps(2) infinite;
}

@keyframes blink {
  50% {
    opacity: 0;
  }
}

.sources {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed #e3e7f5;
}

.src-label {
  font-size: 12px;
  color: #7a839b;
  margin-right: 6px;
}

.src-tag {
  margin-right: 6px;
  margin-top: 4px;
}

.input-row {
  display: flex;
  gap: 10px;
  padding: 12px 16px 6px;
  border-top: 1px solid #f2f4f9;
}

.send-btn {
  align-self: flex-end;
  height: 40px;
  padding: 0 22px;
}

.disclaimer {
  text-align: center;
  font-size: 11.5px;
  color: #b6bdcf;
  padding-bottom: 8px;
}
</style>
